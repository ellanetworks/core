// Copyright 2024 Ella Networks
package core

import (
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

func ParseSdfFilter(flowDescription string) (ebpf.SdfFilter, error) {
	re := regexp.MustCompile(`^permit out (icmp|ip|tcp|udp|\d+) from (any|[\d.]+|[\da-fA-F:]+)(?:/(\d+))?(?: (\d+|\d+-\d+))? to (assigned|any|[\d.]+|[\da-fA-F:]+)(?:/(\d+))?(?: (\d+|\d+-\d+))?$`)

	sdfInfo := ebpf.SdfFilter{}
	var err error

	match := re.FindStringSubmatch(flowDescription)
	logger.UpfLog.Info("Matched groups", zap.Strings("match", match))
	if len(match) == 0 {
		return ebpf.SdfFilter{}, fmt.Errorf("SDF Filter: bad formatting. Should be compatible with regexp: %s", re.String())
	}

	if sdfInfo.Protocol, err = ParseProtocol(match[1]); err != nil {
		return ebpf.SdfFilter{}, err
	}
	if match[2] == "any" {
		if match[3] != "" {
			return ebpf.SdfFilter{}, fmt.Errorf("<any> keyword should not be used with </mask>")
		}
		sdfInfo.SrcAddress = ebpf.IPWMask{Type: 0}
	} else {
		if sdfInfo.SrcAddress, err = ParseCidrIP(match[2], match[3]); err != nil {
			return ebpf.SdfFilter{}, err
		}
	}
	sdfInfo.SrcPortRange = ebpf.PortRange{LowerBound: 0, UpperBound: 65535}
	if match[4] != "" {
		if sdfInfo.SrcPortRange, err = ParsePortRange(match[4]); err != nil {
			return ebpf.SdfFilter{}, err
		}
	}
	if match[5] == "assigned" || match[5] == "any" {
		sdfInfo.DstAddress = ebpf.IPWMask{Type: 0}
	} else {
		if sdfInfo.DstAddress, err = ParseCidrIP(match[5], match[6]); err != nil {
			return ebpf.SdfFilter{}, err
		}
	}
	sdfInfo.DstPortRange = ebpf.PortRange{LowerBound: 0, UpperBound: 65535}
	if match[7] != "" {
		if sdfInfo.DstPortRange, err = ParsePortRange(match[7]); err != nil {
			return ebpf.SdfFilter{}, err
		}
	}

	return sdfInfo, nil
}

func ParseProtocol(protocol string) (uint8, error) {
	if protocol == "58" {
		protocol = "icmp6"
	}
	protocolMap := map[string]uint8{
		"icmp":  0,
		"ip":    1,
		"tcp":   2,
		"udp":   3,
		"icmp6": 4,
	}
	number, ok := protocolMap[protocol]
	if ok {
		return number, nil
	} else {
		return 0, fmt.Errorf("bad protocol formatting")
	}
}

func ParseCidrIP(ipStr, maskStr string) (ebpf.IPWMask, error) {
	var ipType uint8
	if ip := net.ParseIP(ipStr); ip != nil {
		if ip.To4() != nil {
			ipType = 1
			ip = ip.To4()
		} else {
			ipType = 2
		}
		mask := net.CIDRMask(8*len(ip), 8*len(ip))
		if maskStr != "" {
			if maskInt, err := strconv.ParseInt(maskStr, 10, 32); err == nil {
				mask = net.CIDRMask(int(maskInt), 8*len(ip))
				ip = ip.Mask(mask)
			} else {
				return ebpf.IPWMask{}, fmt.Errorf("bad mask formatting")
			}
		}
		return ebpf.IPWMask{
			Type: ipType,
			IP:   ip,
			Mask: mask,
		}, nil
	} else {
		return ebpf.IPWMask{}, fmt.Errorf("bad IP formatting")
	}
}

func ParsePortRange(str string) (ebpf.PortRange, error) {
	re := regexp.MustCompile(`^(\d+)(?:-(\d+))?$`)
	match := re.FindStringSubmatch(str)
	portRange := ebpf.PortRange{}
	var err error
	if portRange.LowerBound, err = ParsePort(match[1]); err != nil {
		return ebpf.PortRange{}, err
	}
	if match[2] != "" {
		if portRange.UpperBound, err = ParsePort(match[2]); err != nil {
			return ebpf.PortRange{}, err
		}
	} else {
		portRange.UpperBound = portRange.LowerBound
	}
	if portRange.LowerBound > portRange.UpperBound {
		return ebpf.PortRange{}, fmt.Errorf("invalid port range: left port should be less or equal to right port")
	}
	return portRange, nil
}

func ParsePort(str string) (uint16, error) {
	if port64, err := strconv.ParseUint(str, 10, 64); err == nil {
		if port64 > 65535 {
			return 0, fmt.Errorf("invalid port: port must be inside bounds [0, 65535]")
		}
		return uint16(port64), nil
	} else {
		return 0, err
	}
}
