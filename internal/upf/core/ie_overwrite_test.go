// Copyright 2024 Ella Networks
package core

import (
	"net"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

const (
	RemoteIP   = "127.0.0.1"
	UeAddress1 = "1.1.1.1"
	UeAddress2 = "2.2.2.2"
	NodeId     = "test-node"
)

type MapOperationsMock struct{}

func (mapOps *MapOperationsMock) PutPdrUplink(teid uint32, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) PutPdrDownlink(ipv4 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) UpdatePdrUplink(teid uint32, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) UpdatePdrDownlink(ipv4 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeletePdrUplink(teid uint32) error {
	return nil
}

func (mapOps *MapOperationsMock) DeletePdrDownlink(ipv4 net.IP) error {
	return nil
}

func (mapOps *MapOperationsMock) PutDownlinkPdrIp6(ipv6 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) UpdateDownlinkPdrIp6(ipv6 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeleteDownlinkPdrIp6(ipv6 net.IP) error {
	return nil
}

func (mapOps *MapOperationsMock) NewFar(farInfo ebpf.FarInfo) (uint32, error) {
	return 0, nil
}

func (mapOps *MapOperationsMock) UpdateFar(internalId uint32, farInfo ebpf.FarInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeleteFar(internalId uint32) error {
	return nil
}

func (mapOps *MapOperationsMock) NewQer(qerInfo ebpf.QerInfo) (uint32, error) {
	return 0, nil
}

func (mapOps *MapOperationsMock) UpdateQer(internalId uint32, qerInfo ebpf.QerInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeleteQer(internalId uint32) error {
	return nil
}
