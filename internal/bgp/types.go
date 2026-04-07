package bgp

import (
	"context"
	"net/netip"
)

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
	Address          string `json:"address"`
	RemoteAS         int    `json:"remoteAS"`
	State            string `json:"state"` // "established", "active", "connect", "idle", etc.
	Uptime           string `json:"uptime,omitempty"`
	PrefixesSent     int    `json:"prefixesSent"`
	PrefixesReceived int    `json:"prefixesReceived"`
}

// BGPRoute represents a single advertised route.
type BGPRoute struct {
	Subscriber string `json:"subscriber"`
	Prefix     string `json:"prefix"`
	NextHop    string `json:"nextHop"`
}

// LearnedRoute represents a BGP-learned route installed in the kernel.
type LearnedRoute struct {
	Prefix  string `json:"prefix"`
	NextHop string `json:"nextHop"`
	Peer    string `json:"peer"`
}

// ImportPrefixEntry is a raw import prefix list entry (string-based, as stored in the DB).
type ImportPrefixEntry struct {
	Prefix    string
	MaxLength int
}

// ImportPrefixStore provides access to per-peer import prefix lists.
type ImportPrefixStore interface {
	ListImportPrefixes(ctx context.Context, peerID int) ([]ImportPrefixEntry, error)
}

// BGPAnnouncer is the interface used by SMF to announce/withdraw routes.
type BGPAnnouncer interface {
	Announce(ip netip.Addr, owner string) error
	Withdraw(ip netip.Addr) error
	IsRunning() bool
	IsAdvertising() bool
}
