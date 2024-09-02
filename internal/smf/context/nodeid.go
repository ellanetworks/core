// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package context

import (
	"fmt"
	"net"
	"time"

	"github.com/yeastengine/ella/internal/smf/logger"
)

const (
	NodeIdTypeIpv4Address uint8 = iota
	NodeIdTypeIpv6Address
	NodeIdTypeFqdn
)

type NodeID struct {
	NodeIdValue []byte
	NodeIdType  uint8 // 0x00001111
}

var dnsHostIpCache map[string]net.IP

func NewNodeID(nodeID string) *NodeID {
	ip := net.ParseIP(nodeID)
	if ip == nil {
		return &NodeID{
			NodeIdType:  NodeIdTypeFqdn,
			NodeIdValue: []byte(nodeID),
		}
	} else if ip.To4() != nil {
		return &NodeID{
			NodeIdType:  NodeIdTypeIpv4Address,
			NodeIdValue: ip.To4(),
		}
	} else {
		return &NodeID{
			NodeIdType:  NodeIdTypeIpv6Address,
			NodeIdValue: ip.To16(),
		}
	}
}

func (n *NodeID) ResolveNodeIdToIp() net.IP {
	switch n.NodeIdType {
	case NodeIdTypeIpv4Address, NodeIdTypeIpv6Address:
		return n.NodeIdValue
	case NodeIdTypeFqdn:
		if ip, err := getDnsHostIp(string(n.NodeIdValue)); err != nil {
			logger.CtxLog.Warnf("host [%v] not found in smf dns cache ", string(n.NodeIdValue))
			if ns, err := net.LookupHost(string(n.NodeIdValue)); err != nil {
				logger.CtxLog.Warnf("host lookup failed: %+v", err)
				return net.IPv4zero
			} else {
				logger.CtxLog.Infof("host [%v] dns resolved, updating smf dns cache ", string(n.NodeIdValue))
				InsertDnsHostIp(string(n.NodeIdValue), net.ParseIP(ns[0]))
				return net.ParseIP(ns[0])
			}
		} else {
			logger.CtxLog.Debugf("host [%v] found in smf dns cache ", string(n.NodeIdValue))
			return ip
		}
	default:
		return net.IPv4zero
	}
}

func init() {
	dnsHostIpCache = make(map[string]net.IP)
	ticker := time.NewTicker(time.Minute)

	go func() {
		for {
			<-ticker.C
			RefreshDnsHostIpCache()
		}
	}()
}

func RefreshDnsHostIpCache() {
	for hostName := range dnsHostIpCache {
		logger.CtxLog.Debugf("refreshing DNS for host [%v] ", hostName)
		if ns, err := net.LookupHost(hostName); err != nil {
			logger.CtxLog.Warnf("host lookup failed: %+v", err)
			deleteDnsHost(hostName)
			continue
		} else if !dnsHostIpCache[hostName].Equal(net.ParseIP(ns[0])) {
			logger.CtxLog.Infof("smf dns cache updated for host [%v]: [%v] ", hostName, net.ParseIP(ns[0]).String())
			dnsHostIpCache[hostName] = net.ParseIP(ns[0])
		}
	}
}

func getDnsHostIp(hostName string) (net.IP, error) {
	if ip, ok := dnsHostIpCache[hostName]; !ok {
		return nil, fmt.Errorf("host [%v] not found in smf dns cache", hostName)
	} else {
		return ip, nil
	}
}

func InsertDnsHostIp(hostName string, ip net.IP) {
	dnsHostIpCache[hostName] = ip
}

func deleteDnsHost(hostName string) {
	delete(dnsHostIpCache, hostName)
}
