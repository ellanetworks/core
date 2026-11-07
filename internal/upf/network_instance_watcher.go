// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	instanceWatchBackstopInterval = time.Minute
	instanceWatchRetryInterval    = time.Second
	instanceWatchRetryMaxInterval = time.Minute
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

	return ni, datapathIface{index: attach.Index, name: st.attachName}, true
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

		delete(u.instanceLinks, index)
	}
}

func (u *UPF) startNetworkInstanceWatch(ctx context.Context) {
	if len(u.instances) == 0 {
		return
	}

	cctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	u.instanceWatchCancel = cancel
	u.instanceWatchDone = done

	go func() {
		defer close(done)

		u.watchNetworkInstances(cctx)
	}()
}

func (u *UPF) stopNetworkInstanceWatch() {
	if u.instanceWatchCancel == nil {
		return
	}

	u.instanceWatchCancel()
	<-u.instanceWatchDone

	u.instanceWatchCancel = nil
}

func (u *UPF) watchNetworkInstances(ctx context.Context) {
	backoff := instanceWatchRetryInterval

	for {
		updates := make(chan netlink.LinkUpdate, 64)
		done := make(chan struct{})

		if err := subscribeLinks(updates, done); err != nil {
			logger.UpfLog.Error("netlink link subscription failed; retrying", zap.Error(err), zap.Duration("retry_in", backoff))
		} else {
			backoff = instanceWatchRetryInterval

			if u.watchNetworkInstanceStream(ctx, done, updates) {
				return
			}

			logger.UpfLog.Warn("netlink link subscription ended; resubscribing", zap.Duration("retry_in", backoff))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, instanceWatchRetryMaxInterval)
	}
}

func (u *UPF) watchNetworkInstanceStream(ctx context.Context, done chan struct{}, updates chan netlink.LinkUpdate) bool {
	defer close(done)

	backstop := time.NewTicker(instanceWatchBackstopInterval)
	defer backstop.Stop()

	u.refreshNetworkInstances()

	for {
		select {
		case <-ctx.Done():
			return true
		case _, ok := <-updates:
			if !ok || !drainLinkUpdates(updates) {
				return false
			}

			u.refreshNetworkInstances()
		case <-backstop.C:
			u.refreshNetworkInstances()
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
