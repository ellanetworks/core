package bgp

import "net"

// BGPSettings holds the configuration for the BGP speaker.
type BGPSettings struct {
	Enabled       bool
	LocalAS       int
	RouterID      string
	ListenAddress string
}

// BGPPeer holds the configuration for a single BGP peer.
type BGPPeer struct {
	ID          int
	Address     string
	RemoteAS    int
	HoldTime    int
	Password    string
	Description string
}

// BGPStatus represents the live state of the BGP speaker's peer sessions.
type BGPStatus struct {
	Peers []BGPPeerStatus `json:"peers,omitempty"`
}

// BGPPeerStatus represents the live state of a BGP peer session.
type BGPPeerStatus struct {
	Address      string `json:"address"`
	RemoteAS     int    `json:"remoteAS"`
	State        string `json:"state"` // "established", "active", "connect", "idle", etc.
	Uptime       string `json:"uptime,omitempty"`
	PrefixesSent int    `json:"prefixesSent"`
}

// BGPRoute represents a single advertised route.
type BGPRoute struct {
	Subscriber string `json:"subscriber"`
	Prefix     string `json:"prefix"`
	NextHop    string `json:"nextHop"`
}

// BGPAnnouncer is the interface used by SMF to announce/withdraw routes.
type BGPAnnouncer interface {
	Announce(ip net.IP, owner string) error
	Withdraw(ip net.IP) error
	IsRunning() bool
	IsAdvertising() bool
}
