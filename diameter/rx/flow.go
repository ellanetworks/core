// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

type Protocol uint16

const (
	ProtocolTCP Protocol = 6
	ProtocolUDP Protocol = 17
	ProtocolIP  Protocol = 256
)

func (p Protocol) String() string {
	if p == ProtocolIP {
		return "ip"
	}

	return strconv.Itoa(int(p))
}

type FlowDescription struct {
	Direction       Direction
	Protocol        Protocol
	Source          netip.Prefix
	SourcePort      uint16
	Destination     netip.Prefix
	DestinationPort uint16
}

var ErrInvalidFlowDescription = errors.New("rx: invalid flow description")

func ParseFlowDescription(s string) (FlowDescription, error) {
	var f FlowDescription
	if err := f.UnmarshalText([]byte(s)); err != nil {
		return FlowDescription{}, err
	}

	return f, nil
}

func (f FlowDescription) String() string {
	b, err := f.MarshalText()
	if err != nil {
		return fmt.Sprintf("FlowDescription(%v)", err)
	}

	return string(b)
}

func (f FlowDescription) MarshalText() ([]byte, error) {
	switch {
	case f.Direction > DirectionOut:
		return nil, flowError("direction %d", f.Direction)
	case f.Protocol > ProtocolIP:
		return nil, flowError("protocol %d", f.Protocol)
	}

	return []byte("permit " + f.Direction.String() + " " + f.Protocol.String() +
		" from " + flowEndpoint(f.Source, f.SourcePort) + " to " + flowEndpoint(f.Destination, f.DestinationPort)), nil
}

func (f *FlowDescription) UnmarshalText(text []byte) error {
	fields := strings.Fields(string(text))
	if len(fields) < 7 || fields[0] != "permit" || fields[3] != "from" {
		return flowError("%q is not a permit rule", text)
	}

	var parsed FlowDescription

	switch fields[1] {
	case "in":
		parsed.Direction = DirectionIn
	case "out":
		parsed.Direction = DirectionOut
	default:
		return flowError("direction %q", fields[1])
	}

	protocol, err := parseProtocol(fields[2])
	if err != nil {
		return err
	}

	parsed.Protocol = protocol

	rest := fields[4:]
	if parsed.Source, parsed.SourcePort, rest, err = parseEndpoint(rest); err != nil {
		return err
	}

	if len(rest) == 0 || rest[0] != "to" {
		return flowError("%q has no destination", text)
	}

	if parsed.Destination, parsed.DestinationPort, rest, err = parseEndpoint(rest[1:]); err != nil {
		return err
	}

	if len(rest) != 0 {
		return flowError("options %q", strings.Join(rest, " "))
	}

	*f = parsed

	return nil
}

func parseProtocol(s string) (Protocol, error) {
	if s == "ip" {
		return ProtocolIP, nil
	}

	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, flowError("protocol %q", s)
	}

	return Protocol(n), nil
}

func parseEndpoint(fields []string) (netip.Prefix, uint16, []string, error) {
	if len(fields) == 0 {
		return netip.Prefix{}, 0, nil, flowError("missing address")
	}

	var prefix netip.Prefix

	if fields[0] != "any" {
		p, err := parseAddress(fields[0])
		if err != nil {
			return netip.Prefix{}, 0, nil, err
		}

		prefix = p
	}

	fields = fields[1:]
	if len(fields) == 0 || fields[0] == "to" {
		return prefix, 0, fields, nil
	}

	port, err := strconv.ParseUint(fields[0], 10, 16)
	if err != nil || port == 0 {
		return netip.Prefix{}, 0, nil, flowError("port %q", fields[0])
	}

	return prefix, uint16(port), fields[1:], nil
}

func parseAddress(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Prefix{}, flowError("address %q", s)
		}

		return p, nil
	}

	a, err := netip.ParseAddr(s)
	if err != nil || a.Zone() != "" {
		return netip.Prefix{}, flowError("address %q", s)
	}

	return netip.PrefixFrom(a, a.BitLen()), nil
}

func flowEndpoint(p netip.Prefix, port uint16) string {
	s := "any"

	switch {
	case p.IsSingleIP():
		s = p.Addr().String()
	case p.IsValid():
		s = p.String()
	}

	if port != 0 {
		s += " " + strconv.Itoa(int(port))
	}

	return s
}

func flowError(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidFlowDescription}, args...)...)
}
