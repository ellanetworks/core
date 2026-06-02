package ebpf

// Veth XDP program for the veth-smf <-> veth-xdp injection path.

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf VethEntrypoint xdp/veth_bpf.c -- -I. -O2 -Wall -Werror -g

import (
	"errors"
	"fmt"
	"net/netip"
	"runtime"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// VethBpfObjects wraps the loaded veth XDP program and its maps.
type VethBpfObjects struct {
	VethEntrypointObjects
}

// LoadVethBpfObjects loads the veth XDP program and maps into the kernel. The
// routing constants must be configured: veth_xdp_func reuses route_ipv4/
// route_ipv6, and the IPv4 transport path redirects to n3_ifindex — left unset
// it routes the encapsulated Router Advertisement to interface 0 and drops it.
func LoadVethBpfObjects(masquerade bool, n3Ifindex, n6Ifindex int, n3Vlan, n6Vlan uint32) (*VethBpfObjects, error) {
	spec, err := LoadVethEntrypoint()
	if err != nil {
		return nil, fmt.Errorf("load veth entrypoint spec: %w", err)
	}

	if m, ok := spec.Maps["csum_scratch"]; ok {
		m.MaxEntries = uint32(runtime.NumCPU())
	}

	if err := spec.Variables["masquerade"].Set(masquerade); err != nil {
		return nil, fmt.Errorf("set masquerade: %w", err)
	}

	if err := spec.Variables["n3_ifindex"].Set(uint32(n3Ifindex)); err != nil {
		return nil, fmt.Errorf("set n3_ifindex: %w", err)
	}

	if err := spec.Variables["n6_ifindex"].Set(uint32(n6Ifindex)); err != nil {
		return nil, fmt.Errorf("set n6_ifindex: %w", err)
	}

	if err := spec.Variables["n3_vlan"].Set(n3Vlan); err != nil {
		return nil, fmt.Errorf("set n3_vlan: %w", err)
	}

	if err := spec.Variables["n6_vlan"].Set(n6Vlan); err != nil {
		return nil, fmt.Errorf("set n6_vlan: %w", err)
	}

	var objs VethEntrypointObjects
	if err := spec.LoadAndAssign(&objs, &ebpf.CollectionOptions{}); err != nil {
		logger.UpfLog.Error("failed to load veth XDP program", zap.Error(err))

		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			logger.UpfLog.Debug("verifier log", zap.String("verifier", fmt.Sprintf("%+v", ve)))
		}

		return nil, fmt.Errorf("load veth XDP program: %w", err)
	}

	return &VethBpfObjects{VethEntrypointObjects: objs}, nil
}

// Close releases all kernel resources held by the veth BPF objects.
func (v *VethBpfObjects) Close() error {
	return v.VethEntrypointObjects.Close()
}

// VethTunnelInfo holds the Go-side representation of a veth tunnel map entry.
type VethTunnelInfo struct {
	TEID       uint32
	LocalAddr  netip.Addr // UPF N3 transport address (IPv4 or IPv6)
	RemoteAddr netip.Addr // gNB N3 transport address (IPv4 or IPv6)
	QFI        uint8
}

// PutTunnel programs a tunnel entry in the veth_tunnels BPF map.
// The key is the inner IPv6 destination address.
func (v *VethBpfObjects) PutTunnel(dstIPv6 netip.Addr, info VethTunnelInfo) error {
	key := VethEntrypointIn6Addr{}
	key.In6U.U6Addr8 = dstIPv6.As16()

	localAddr := VethEntrypointIn6Addr{}
	localAddr.In6U.U6Addr8 = IPToIn6Addr(info.LocalAddr)

	remoteAddr := VethEntrypointIn6Addr{}
	remoteAddr.In6U.U6Addr8 = IPToIn6Addr(info.RemoteAddr)

	val := VethEntrypointVethTunnelInfo{
		Teid:       info.TEID,
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Qfi:        info.QFI,
	}

	return v.VethTunnels.Put(&key, &val)
}

// DeleteTunnel removes a tunnel entry from the veth_tunnels BPF map.
func (v *VethBpfObjects) DeleteTunnel(dstIPv6 netip.Addr) error {
	key := VethEntrypointIn6Addr{}
	key.In6U.U6Addr8 = dstIPv6.As16()

	return v.VethTunnels.Delete(&key)
}
