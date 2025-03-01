// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

const ProtocolNumberAny = 0xfc

// Action - Action of IPFilterRule
type Action string

// Action const
const (
	Permit Action = "permit"
	Deny   Action = "deny"
)

// Direction - direction of IPFilterRule
type Direction string

// Direction const
const (
	In  Direction = "in"
	Out Direction = "out"
)

func flowDescErrorf(format string, a ...interface{}) error {
	msg := fmt.Sprintf(format, a...)
	return fmt.Errorf("flowdesc: %s", msg)
}

// IPFilterRule define RFC 3588 that referd by TS 29.212
type IPFilterRule struct {
	action   Action
	dir      Direction
	proto    uint8  // protocol number
	srcIP    string // <address/mask>
	srcPorts string // [ports]
	dstIP    string // <address/mask>
	dstPorts string // [ports]
}

// NewIPFilterRule returns a new IPFilterRule instance
func NewIPFilterRule() *IPFilterRule {
	r := &IPFilterRule{
		action:   Permit,
		dir:      Out,
		proto:    ProtocolNumberAny,
		srcIP:    "",
		srcPorts: "",
		dstIP:    "",
		dstPorts: "",
	}
	return r
}

// SetAction sets action of the IPFilterRule
func (r *IPFilterRule) SetAction(action Action) error {
	switch action {
	case Permit:
		r.action = action
	case Deny:
		r.action = action
	default:
		return flowDescErrorf("'%s' is not allow, action only accept 'permit' or 'deny'", action)
	}
	return nil
}

// SetDirection sets direction of the IPFilterRule
func (r *IPFilterRule) SetDirection(dir Direction) error {
	switch dir {
	case Out:
		r.dir = dir
	case In:
		return flowDescErrorf("dir cannot be 'in' in core-network")
	default:
		return flowDescErrorf("'%s' is not allow, dir only accept 'out'", dir)
	}
	return nil
}

// SetSourceIP sets source IP of the IPFilterRule
func (r *IPFilterRule) SetSourceIP(networkStr string) error {
	if networkStr == "" {
		return flowDescErrorf("Empty string")
	}
	if networkStr == "any" || networkStr == "assigned" {
		r.srcIP = networkStr
		return nil
	}
	if networkStr[0] == '!' {
		return flowDescErrorf("Base on TS 29.212, ! expression shall not be used")
	}

	var ipStr string

	ip := net.ParseIP(networkStr)
	if ip == nil {
		_, ipNet, err := net.ParseCIDR(networkStr)
		if err != nil {
			return flowDescErrorf("Source IP format error")
		}
		ipStr = ipNet.String()
	} else {
		ipStr = ip.String()
	}

	r.srcIP = ipStr
	return nil
}

// SetDestinationIP sets destination IP of the IPFilterRule
func (r *IPFilterRule) SetDestinationIP(networkStr string) error {
	if networkStr == "any" || networkStr == "assigned" {
		r.dstIP = networkStr
		return nil
	}
	if networkStr[0] == '!' {
		return flowDescErrorf("Base on TS 29.212, ! expression shall not be used")
	}

	var ipDst string

	ip := net.ParseIP(networkStr)
	if ip == nil {
		_, ipNet, err := net.ParseCIDR(networkStr)
		if err != nil {
			return flowDescErrorf("Source IP format error")
		}
		ipDst = ipNet.String()
	} else {
		ipDst = ip.String()
	}

	r.dstIP = ipDst
	return nil
}

// SetDestinationPorts sets destination ports of the IPFilterRule
func (r *IPFilterRule) SetDestinationPorts(ports string) error {
	if ports == "" {
		r.dstPorts = ports
		return nil
	}

	match, err := regexp.MatchString("^[0-9]+(-[0-9]+)?(,[0-9]+)*$", ports)
	if err != nil {
		return flowDescErrorf("Regex match error")
	}
	if !match {
		return flowDescErrorf("Ports format error")
	}

	// Check port range
	portSlice := regexp.MustCompile(`[\\,\\-]+`).Split(ports, -1)
	for _, portStr := range portSlice {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return err
		}
		if port < 0 || port > 65535 {
			return flowDescErrorf("Invalid port number")
		}
	}

	r.dstPorts = ports
	return nil
}

// Encode function out put the IPFilterRule from the struct
func Encode(r *IPFilterRule) (string, error) {
	var ipFilterRuleStr []string

	// pre-allocate seven element
	ipFilterRuleStr = make([]string, 0, 9)

	// action
	switch r.action {
	case Permit:
		ipFilterRuleStr = append(ipFilterRuleStr, "permit")
	case Deny:
		ipFilterRuleStr = append(ipFilterRuleStr, "deny")
	}

	// dir
	switch r.dir {
	case Out:
		ipFilterRuleStr = append(ipFilterRuleStr, "out")
	}

	// proto
	if r.proto == ProtocolNumberAny {
		ipFilterRuleStr = append(ipFilterRuleStr, "ip")
	} else {
		ipFilterRuleStr = append(ipFilterRuleStr, strconv.Itoa(int(r.proto)))
	}

	// from
	ipFilterRuleStr = append(ipFilterRuleStr, "from")

	// src
	if r.srcIP != "" {
		ipFilterRuleStr = append(ipFilterRuleStr, r.srcIP)
	} else {
		ipFilterRuleStr = append(ipFilterRuleStr, "any")
	}
	if r.srcPorts != "" {
		ipFilterRuleStr = append(ipFilterRuleStr, r.srcPorts)
	}

	// to
	ipFilterRuleStr = append(ipFilterRuleStr, "to")

	// dst
	if r.dstIP != "" {
		ipFilterRuleStr = append(ipFilterRuleStr, r.dstIP)
	} else {
		ipFilterRuleStr = append(ipFilterRuleStr, "any")
	}
	if r.dstPorts != "" {
		ipFilterRuleStr = append(ipFilterRuleStr, r.dstPorts)
	}

	// according TS 29.212 IPFilterRule cannot use [options]

	return strings.Join(ipFilterRuleStr, " "), nil
}
