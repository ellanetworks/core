// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	linkWatchBackstopInterval = time.Minute
	linkWatchRetryInterval    = time.Second
	linkWatchRetryMaxInterval = time.Minute
)

type networkInstanceState struct {
	name        string
	attachName  string
	routingName string
	id          uint32
	vlan        uint32
	current     ebpf.NetworkInstance
}

var (
	attachInstanceLink    = attachInterface
	putNetworkInstance    = (*ebpf.BpfObjects).PutNetworkInstance
	deleteNetworkInstance = (*ebpf.BpfObjects).DeleteNetworkInstance
	releaseInstanceLink   = (*ebpf.BpfObjects).ReleaseInterface
)

var subscribeLinks = func(updates chan<- netlink.LinkUpdate, done <-chan struct{}) error {
	return netlink.LinkSubscribe(updates, done)
}

func newNetworkInstanceStates(resolved []resolvedNetworkInstance) []*networkInstanceState {
	states := make([]*networkInstanceState, 0, len(resolved))

	for _, ni := range resolved {
		states = append(states, &networkInstanceState{
			name:        ni.name,
			attachName:  ni.attachName,
			routingName: ni.interfaceName,
			id:          ni.datapath.ID,
			vlan:        ni.datapath.Vlan,
			current:     ni.datapath,
		})
	}

	return states
}

func (st *networkInstanceState) lookup() (ebpf.NetworkInstance, datapathIface, bool) {
	attach, err := interfaceByName(st.attachName)
	if err != nil {
		return ebpf.NetworkInstance{}, datapathIface{}, false
	}

	routing, err := interfaceByName(st.routingName)
	if err != nil {
		return ebpf.NetworkInstance{}, datapathIface{}, false
	}

	ni := ebpf.NetworkInstance{
		ID:             st.id,
		Ifindex:        uint32(attach.Index),
		RoutingIfindex: uint32(routing.Index),
		Vlan:           st.vlan,
	}

	return ni, datapathIface{index: attach.Index, name: st.attachName, tagged: st.vlan != 0}, true
}

func (u *UPF) availableNetworkInstances() map[string]uint8 {
	ids := make(map[string]uint8, len(u.instances))

	for _, st := range u.instances {
		if st.current.Ifindex != 0 {
			ids[st.name] = uint8(st.id)
		}
	}

	return ids
}

func (u *UPF) refreshNetworkInstances() {
	u.instanceMu.Lock()
	defer u.instanceMu.Unlock()

	if u.instanceLinks == nil {
		return
	}

	changed := false

	for _, st := range u.instances {
		want, iface, ok := st.lookup()
		if ok && want == st.current {
			continue
		}

		if !ok && st.current.Ifindex == 0 {
			continue
		}

		if st.current.Ifindex != 0 {
			if err := deleteNetworkInstance(u.se.BpfObjects, st.current); err != nil {
				logger.UpfLog.Warn("failed to remove a network instance from the datapath",
					zap.String("network_instance", st.name), zap.Error(err))
			}

			st.current = ebpf.NetworkInstance{}
			changed = true
		}

		if !ok {
			logger.UpfLog.Warn("network instance unavailable: its interface is gone, new sessions on it are rejected",
				zap.String("network_instance", st.name), zap.String("n6", st.routingName))

			continue
		}

		if err := u.attachInstanceInterface(iface); err != nil {
			logger.UpfLog.Warn("network instance unavailable: cannot attach the datapath to its interface",
				zap.String("network_instance", st.name), zap.String("iface", iface.name), zap.Error(err))

			continue
		}

		if err := putNetworkInstance(u.se.BpfObjects, want); err != nil {
			logger.UpfLog.Warn("network instance unavailable: cannot add it to the datapath",
				zap.String("network_instance", st.name), zap.Error(err))

			continue
		}

		st.current = want
		changed = true

		logger.UpfLog.Info("network instance attached", zap.String("network_instance", st.name), zap.String("n6", st.routingName))
	}

	u.detachUnusedInstanceInterfaces()

	if changed {
		u.se.SetNetworkInstances(u.availableNetworkInstances())
	}
}

func (u *UPF) attachInstanceInterface(iface datapathIface) error {
	if u.datapathIfindex[iface.index] {
		return nil
	}

	if _, ok := u.instanceLinks[iface.index]; ok {
		return nil
	}

	l, err := attachInstanceLink(u.se.BpfObjects, u.attachedMode, iface)
	if err != nil {
		return err
	}

	warnVLANStrippedBeforeXDP(iface, u.attachedMode)

	u.instanceLinks[iface.index] = l

	return nil
}

func (u *UPF) detachUnusedInstanceInterfaces() {
	used := make(map[int]bool, len(u.instances))

	for _, st := range u.instances {
		if st.current.Ifindex != 0 {
			used[int(st.current.Ifindex)] = true
		}
	}

	for index, l := range u.instanceLinks {
		if used[index] {
			continue
		}

		_ = l.Close()

		releaseInstanceLink(u.se.BpfObjects, index)

		delete(u.instanceLinks, index)
	}
}

func (u *UPF) startLinkWatch(ctx context.Context) {
	if len(u.instances) == 0 && !mergingEnforced(u.attachedMode) {
		return
	}

	cctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	u.linkWatchCancel = cancel
	u.linkWatchDone = done

	go func() {
		defer close(done)

		u.watchLinks(cctx)
	}()
}

func (u *UPF) stopLinkWatch() {
	if u.linkWatchCancel == nil {
		return
	}

	u.linkWatchCancel()
	<-u.linkWatchDone

	u.linkWatchCancel = nil
}

func (u *UPF) watchLinks(ctx context.Context) {
	backoff := linkWatchRetryInterval

	for {
		updates := make(chan netlink.LinkUpdate, 64)
		done := make(chan struct{})

		if err := subscribeLinks(updates, done); err != nil {
			logger.UpfLog.Error("netlink link subscription failed; retrying", zap.Error(err), zap.Duration("retry_in", backoff))
		} else {
			backoff = linkWatchRetryInterval

			if u.watchLinkStream(ctx, done, updates) {
				return
			}

			logger.UpfLog.Warn("netlink link subscription ended; resubscribing", zap.Duration("retry_in", backoff))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, linkWatchRetryMaxInterval)
	}
}

func (u *UPF) watchLinkStream(ctx context.Context, done chan struct{}, updates chan netlink.LinkUpdate) bool {
	defer close(done)

	backstop := time.NewTicker(linkWatchBackstopInterval)
	defer backstop.Stop()

	u.refreshLinks()

	for {
		select {
		case <-ctx.Done():
			return true
		case _, ok := <-updates:
			if !ok || !drainLinkUpdates(updates) {
				return false
			}

			u.refreshLinks()
		case <-backstop.C:
			u.refreshLinks()
		}
	}
}

func drainLinkUpdates(updates chan netlink.LinkUpdate) bool {
	for {
		select {
		case _, ok := <-updates:
			if !ok {
				return false
			}
		default:
			return true
		}
	}
}

func (u *UPF) refreshLinks() {
	if len(u.instances) > 0 {
		u.refreshNetworkInstances()
	}

	u.refreshMerging()
}

func mergingEnforced(mode string) bool {
	return mode == config.DatapathTCX || mode == config.DatapathXDPGeneric
}

func (u *UPF) refreshMerging() {
	u.enforceMerging(true)
}

func (u *UPF) seedMerging() {
	u.enforceMerging(false)
}

func (u *UPF) enforceMerging(logWarnings bool) {
	if !mergingEnforced(u.attachedMode) {
		return
	}

	u.instanceMu.Lock()
	defer u.instanceMu.Unlock()

	if u.mergingWarned == nil {
		u.mergingWarned = make(map[int]bool)
	}

	for _, device := range u.mergingDevicesInUse() {
		l, err := netlink.LinkByName(device)
		if err != nil {
			continue
		}

		index := l.Attrs().Index
		warn := logWarnings && !u.mergingWarned[index]

		if turnOffMerging(device, u.attachedMode, warn) {
			u.mergingWarned[index] = true
		}
	}
}

func (u *UPF) mergingDevicesInUse() []string {
	indexes := []int{u.n3Ifindex, u.n6Ifindex}
	for index := range u.instanceLinks {
		indexes = append(indexes, index)
	}

	var devices []string

	for _, index := range indexes {
		datapath, err := netlink.LinkByIndex(index)
		if err != nil {
			continue
		}

		name := datapath.Attrs().Name
		if _, ok := mergingLowerDevice(name); ok {
			continue
		}

		for _, device := range mergingDevices(u.attachedMode, name) {
			if !slices.Contains(devices, device) {
				devices = append(devices, device)
			}
		}
	}

	return devices
}
