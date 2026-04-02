package models

import "fmt"

// Direction represents the traffic direction for a network rule or filter.
type Direction int

const (
	DirectionUplink   Direction = iota // "uplink": traffic from UE to network
	DirectionDownlink                  // "downlink": traffic from network to UE
)

func (d Direction) String() string {
	switch d {
	case DirectionUplink:
		return "uplink"
	case DirectionDownlink:
		return "downlink"
	default:
		return "unknown"
	}
}

// ParseDirection converts a direction string to a Direction value.
// Returns an error if the string is not a valid direction.
func ParseDirection(s string) (Direction, error) {
	switch s {
	case "uplink":
		return DirectionUplink, nil
	case "downlink":
		return DirectionDownlink, nil
	default:
		return 0, fmt.Errorf("unknown direction %q: must be \"uplink\" or \"downlink\"", s)
	}
}

type Action int

const (
	Allow Action = iota
	Deny
)

func (a Action) String() string {
	if a == Allow {
		return "allow"
	}

	return "deny"
}

func ActionFromString(s string) Action {
	if s == "deny" {
		return Deny
	}

	return Allow
}

type FilterRule struct {
	RemotePrefix string // CIDR notation; "" = any
	Protocol     int32  // 0 = any (maps to SdfProtoAny)
	PortLow      int32
	PortHigh     int32
	Action       Action
}
