// Copyright 2026 Ella Networks

package ndp_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
)

// buildRawRS constructs a raw ICMPv6 Router Solicitation payload with an
// optional Source Link-Layer Address option (6-byte Ethernet MAC).
func buildRawRS(mac []byte) []byte {
	size := 8 // fixed RS header: type + code + checksum + reserved
	if len(mac) > 0 {
		size += 8 // Source Link-Layer Address option: type(1) + len(1) + mac(6)
	}

	buf := make([]byte, size)
	buf[0] = 133 // Type = Router Solicitation
	buf[1] = 0   // Code = 0
	// buf[2:4] = checksum (zero for tests)
	// buf[4:8] = reserved (zero)

	if len(mac) > 0 {
		buf[8] = 1 // Option Type: Source Link-Layer Address
		buf[9] = 1 // Option Length: 1 (in 8-octet units = 8 bytes)
		copy(buf[10:16], mac)
	}

	return buf
}

func TestBuildRA_BasicFields(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:             netip.MustParseAddr("fe80::1"),
		DstIP:             netip.MustParseAddr("fe80::2"),
		CurHopLimit:       64,
		Managed:           false,
		Other:             false,
		RouterLifetime:    1800,
		ReachableTime:     0,
		RetransTimer:      0,
		Prefix:            netip.MustParsePrefix("2001:db8:abcd:1000::/64"),
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     0xFFFFFFFF,
		PreferredLifetime: 3600,
		MTU:               0, // no MTU option
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// Expected size: 16 (RA header) + 32 (Prefix Info Option) = 48
	if len(ra) != 48 {
		t.Fatalf("expected RA length 48, got %d", len(ra))
	}

	// -- Verify RA header fields --
	if ra[0] != 134 {
		t.Errorf("Type: expected 134, got %d", ra[0])
	}

	if ra[1] != 0 {
		t.Errorf("Code: expected 0, got %d", ra[1])
	}
	// Checksum should be zero (caller fills it)
	if binary.BigEndian.Uint16(ra[2:4]) != 0 {
		t.Errorf("Checksum: expected 0, got %d", binary.BigEndian.Uint16(ra[2:4]))
	}

	if ra[4] != 64 {
		t.Errorf("CurHopLimit: expected 64, got %d", ra[4])
	}

	if ra[5] != 0 {
		t.Errorf("Flags: expected 0x00, got 0x%02X", ra[5])
	}

	if binary.BigEndian.Uint16(ra[6:8]) != 1800 {
		t.Errorf("RouterLifetime: expected 1800, got %d", binary.BigEndian.Uint16(ra[6:8]))
	}

	if binary.BigEndian.Uint32(ra[8:12]) != 0 {
		t.Errorf("ReachableTime: expected 0, got %d", binary.BigEndian.Uint32(ra[8:12]))
	}

	if binary.BigEndian.Uint32(ra[12:16]) != 0 {
		t.Errorf("RetransTimer: expected 0, got %d", binary.BigEndian.Uint32(ra[12:16]))
	}
}

func TestBuildRA_ManagedOtherFlags(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:             netip.MustParseAddr("fe80::1"),
		DstIP:             netip.MustParseAddr("fe80::2"),
		CurHopLimit:       64,
		Managed:           true,
		Other:             true,
		RouterLifetime:    0,
		Prefix:            netip.MustParsePrefix("2001:db8::/64"),
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     7200,
		PreferredLifetime: 3600,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// M=1, O=1 -> flags byte = 0xC0
	if ra[5] != 0xC0 {
		t.Errorf("Flags: expected 0xC0, got 0x%02X", ra[5])
	}
}

func TestBuildRA_PrefixInfoOption(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:abcd:1000::/64")
	params := ndp.RAParams{
		SrcIP:             netip.MustParseAddr("fe80::1"),
		DstIP:             netip.MustParseAddr("fe80::2"),
		Prefix:            prefix,
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     0xFFFFFFFF,
		PreferredLifetime: 3600,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// Prefix Information Option starts at offset 16
	pio := ra[16:]

	if pio[0] != 3 { // Type
		t.Errorf("PIO Type: expected 3, got %d", pio[0])
	}

	if pio[1] != 4 { // Length in 8-octet units
		t.Errorf("PIO Length: expected 4, got %d", pio[1])
	}

	if pio[2] != 64 { // Prefix Length
		t.Errorf("PIO PrefixLength: expected 64, got %d", pio[2])
	}
	// L=1, A=1 -> flags byte = 0xC0
	if pio[3] != 0xC0 {
		t.Errorf("PIO Flags: expected 0xC0, got 0x%02X", pio[3])
	}

	if binary.BigEndian.Uint32(pio[4:8]) != 0xFFFFFFFF {
		t.Errorf("PIO ValidLifetime: expected 0xFFFFFFFF, got 0x%08X", binary.BigEndian.Uint32(pio[4:8]))
	}

	if binary.BigEndian.Uint32(pio[8:12]) != 3600 {
		t.Errorf("PIO PreferredLifetime: expected 3600, got %d", binary.BigEndian.Uint32(pio[8:12]))
	}
	// Reserved2 (bytes 12-15) must be zero
	if binary.BigEndian.Uint32(pio[12:16]) != 0 {
		t.Errorf("PIO Reserved2: expected 0, got %d", binary.BigEndian.Uint32(pio[12:16]))
	}

	// Prefix bytes (16-31): should match 2001:0db8:abcd:1000::
	expectedPrefix := prefix.Masked().Addr().As16()
	for i := 0; i < 16; i++ {
		if pio[16+i] != expectedPrefix[i] {
			t.Errorf("PIO Prefix byte %d: expected 0x%02X, got 0x%02X", i, expectedPrefix[i], pio[16+i])
		}
	}
}

func TestBuildRA_PrefixInfoOption_OnLinkOnly(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:      netip.MustParseAddr("fe80::1"),
		DstIP:      netip.MustParseAddr("fe80::2"),
		Prefix:     netip.MustParsePrefix("2001:db8::/64"),
		OnLink:     true,
		Autonomous: false,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// L=1, A=0 -> 0x80
	if ra[16+3] != 0x80 {
		t.Errorf("PIO Flags: expected 0x80 (L only), got 0x%02X", ra[16+3])
	}
}

func TestBuildRA_PrefixInfoOption_AutonomousOnly(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:      netip.MustParseAddr("fe80::1"),
		DstIP:      netip.MustParseAddr("fe80::2"),
		Prefix:     netip.MustParsePrefix("2001:db8::/64"),
		OnLink:     false,
		Autonomous: true,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// L=0, A=1 -> 0x40
	if ra[16+3] != 0x40 {
		t.Errorf("PIO Flags: expected 0x40 (A only), got 0x%02X", ra[16+3])
	}
}

func TestBuildRA_WithMTUOption(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:             netip.MustParseAddr("fe80::1"),
		DstIP:             netip.MustParseAddr("fe80::2"),
		Prefix:            netip.MustParsePrefix("2001:db8::/64"),
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     7200,
		PreferredLifetime: 3600,
		MTU:               1500,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// Expected size: 16 (RA header) + 32 (PIO) + 8 (MTU) = 56
	if len(ra) != 56 {
		t.Fatalf("expected RA length 56, got %d", len(ra))
	}

	// MTU option starts at offset 48
	mtuOpt := ra[48:]
	if mtuOpt[0] != 5 { // Type
		t.Errorf("MTU Type: expected 5, got %d", mtuOpt[0])
	}

	if mtuOpt[1] != 1 { // Length in 8-octet units
		t.Errorf("MTU Length: expected 1, got %d", mtuOpt[1])
	}
	// Reserved (bytes 2-3) must be zero
	if binary.BigEndian.Uint16(mtuOpt[2:4]) != 0 {
		t.Errorf("MTU Reserved: expected 0, got %d", binary.BigEndian.Uint16(mtuOpt[2:4]))
	}

	if binary.BigEndian.Uint32(mtuOpt[4:8]) != 1500 {
		t.Errorf("MTU value: expected 1500, got %d", binary.BigEndian.Uint32(mtuOpt[4:8]))
	}
}

func TestBuildRA_NoMTUOptionWhenZero(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:  netip.MustParseAddr("fe80::1"),
		DstIP:  netip.MustParseAddr("fe80::2"),
		Prefix: netip.MustParsePrefix("2001:db8::/64"),
		MTU:    0,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// No MTU option -> 16 + 32 = 48
	if len(ra) != 48 {
		t.Fatalf("expected RA length 48 (no MTU option), got %d", len(ra))
	}
}

func TestBuildRA_MissingPrefix(t *testing.T) {
	params := ndp.RAParams{
		SrcIP: netip.MustParseAddr("fe80::1"),
		DstIP: netip.MustParseAddr("fe80::2"),
	}

	_, err := ndp.BuildRA(params)
	if err != ndp.ErrMissingPrefix {
		t.Fatalf("expected ErrMissingPrefix, got %v", err)
	}
}

func TestBuildRA_IPv4AddressRejected(t *testing.T) {
	params := ndp.RAParams{
		SrcIP:  netip.MustParseAddr("192.168.1.1"),
		DstIP:  netip.MustParseAddr("fe80::2"),
		Prefix: netip.MustParsePrefix("2001:db8::/64"),
	}

	_, err := ndp.BuildRA(params)
	if err != ndp.ErrNotIPv6 {
		t.Fatalf("expected ErrNotIPv6, got %v", err)
	}
}

func TestICMPv6Checksum(t *testing.T) {
	// Build an RA and compute its checksum, then verify the checksum
	// validates (sum of pseudo-header + message with checksum = 0xFFFF).
	src := netip.MustParseAddr("fe80::1")
	dst := netip.MustParseAddr("fe80::2")

	params := ndp.RAParams{
		SrcIP:             src,
		DstIP:             dst,
		CurHopLimit:       64,
		RouterLifetime:    1800,
		Prefix:            netip.MustParsePrefix("2001:db8:abcd:1000::/64"),
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     0xFFFFFFFF,
		PreferredLifetime: 3600,
		MTU:               1500,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// Checksum field must be zero before computing
	if binary.BigEndian.Uint16(ra[2:4]) != 0 {
		t.Fatalf("checksum field not zero before compute")
	}

	ndp.SetICMPv6Checksum(src, dst, ra)

	csum := binary.BigEndian.Uint16(ra[2:4])
	if csum == 0 {
		t.Fatal("checksum should not be zero after SetICMPv6Checksum")
	}

	// Verify: computing checksum over the message with checksum set
	// should produce 0 (one's complement identity).
	verify := ndp.ICMPv6Checksum(src, dst, ra)
	if verify != 0 {
		t.Fatalf("checksum verification failed: expected 0, got 0x%04X", verify)
	}
}

func TestICMPv6Checksum_KnownRS(t *testing.T) {
	// Verify checksum on a minimal RS.
	src := netip.MustParseAddr("fe80::1")
	dst := netip.MustParseAddr("ff02::2") // all-routers multicast

	rs := buildRawRS(nil) // 8-byte RS, checksum field = 0
	ndp.SetICMPv6Checksum(src, dst, rs)

	csum := binary.BigEndian.Uint16(rs[2:4])
	if csum == 0 {
		t.Fatal("checksum should not be zero")
	}

	// Verify
	verify := ndp.ICMPv6Checksum(src, dst, rs)
	if verify != 0 {
		t.Fatalf("checksum verification failed: expected 0, got 0x%04X", verify)
	}
}

func TestBuildRA_PrefixMasked(t *testing.T) {
	// Passing a prefix with host bits set: "2001:db8::1/64" should
	// produce the same prefix bytes as "2001:db8::/64" (masked).
	params := ndp.RAParams{
		SrcIP:         netip.MustParseAddr("fe80::1"),
		DstIP:         netip.MustParseAddr("fe80::2"),
		Prefix:        netip.MustParsePrefix("2001:db8::1/64"),
		OnLink:        true,
		Autonomous:    true,
		ValidLifetime: 7200,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	// Prefix bytes in PIO (offset 16+16 = 32) should be 2001:0db8:: (no host bits).
	expected := netip.MustParsePrefix("2001:db8::/64").Addr().As16()
	for i := 0; i < 16; i++ {
		if ra[32+i] != expected[i] {
			t.Errorf("PIO Prefix byte %d: expected 0x%02X, got 0x%02X", i, expected[i], ra[32+i])
		}
	}
}

func TestBuildRA_DifferentPrefixLengths(t *testing.T) {
	// The /60 prefix should be encoded with PrefixLength=60 in the PIO.
	params := ndp.RAParams{
		SrcIP:         netip.MustParseAddr("fe80::1"),
		DstIP:         netip.MustParseAddr("fe80::2"),
		Prefix:        netip.MustParsePrefix("2001:db8:abcd:1230::/60"),
		OnLink:        true,
		Autonomous:    true,
		ValidLifetime: 7200,
	}

	ra, err := ndp.BuildRA(params)
	if err != nil {
		t.Fatalf("BuildRA failed: %v", err)
	}

	if ra[16+2] != 60 {
		t.Errorf("PIO PrefixLength: expected 60, got %d", ra[16+2])
	}
}
