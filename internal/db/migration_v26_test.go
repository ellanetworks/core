// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func openV25WithData(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	if err := runMigrations(ctx, conn, 25); err != nil {
		t.Fatalf("migrate to v25: %v", err)
	}

	seed := []string{
		`INSERT INTO profiles (id, name, ueAmbrUplink, ueAmbrDownlink, allow4G, allow5G) VALUES ('pr', 'default', '1 Gbps', '1 Gbps', 1, 1)`,
		`INSERT INTO data_networks (id, name, ipPool, ipv6Pool, dns, mtu) VALUES ('dn', 'internet', '10.45.0.0/16', '', '8.8.8.8', 1400)`,
		`INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn)
		   VALUES ('s1', '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', 'first', '15550001')`,
		`INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn)
		   VALUES ('s2', '001010000000002', '000000000002', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', '', '')`,
		`INSERT INTO daily_usage (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES (20000, '001010000000001', 10, 20)`,
		`INSERT INTO ue_registrations (imsi, type, nodeID, purged, version) VALUES ('001010000000001', 'mme', 'node-1', 0, 1)`,
		`INSERT INTO ip_leases (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType) VALUES ('l1', 'dn', X'0A2D0001', '001010000000001', 5, 'dynamic', 1, 'node-1', 'ipv4')`,
		`INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('smsc-1', 'smsc', 'smsc.example.org', '10.0.0.5', 3868)`,
		`INSERT INTO sms_smsc_service_centres (serviceCentre, peerId) VALUES ('15550000000', 'smsc-1')`,
	}

	for _, stmt := range seed {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := runMigrations(ctx, conn, 26); err != nil {
		t.Fatalf("migrate to v26: %v", err)
	}

	return conn
}

func mustExec(t *testing.T, conn *sql.DB, stmt string) {
	t.Helper()

	if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("%q: %v", stmt, err)
	}
}

func mustFail(t *testing.T, conn *sql.DB, stmt string) {
	t.Helper()

	if _, err := conn.ExecContext(context.Background(), stmt); err == nil {
		t.Errorf("%q was accepted", stmt)
	}
}

func countRows(t *testing.T, conn *sql.DB, query string) int {
	t.Helper()

	var n int

	if err := conn.QueryRowContext(context.Background(), query).Scan(&n); err != nil {
		t.Fatalf("%q: %v", query, err)
	}

	return n
}

func TestMigrationV26PreservesExistingData(t *testing.T) {
	ctx := context.Background()
	conn := openV25WithData(t)

	var (
		imsi, description, msisdn string
		partner                   sql.NullString
	)

	if err := conn.QueryRowContext(ctx, "SELECT imsi, description, msisdn, roamingPartnerID FROM subscribers WHERE id = 's1'").Scan(&imsi, &description, &msisdn, &partner); err != nil {
		t.Fatalf("read subscriber: %v", err)
	}

	if imsi != "001010000000001" || description != "first" || msisdn != "15550001" || partner.Valid {
		t.Fatalf("subscriber after v26 = %s/%s/%s/%v", imsi, description, msisdn, partner)
	}

	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM subscribers":                                                                     2,
		"SELECT COUNT(*) FROM daily_usage":                                                                     1,
		"SELECT COUNT(*) FROM ue_registrations":                                                                1,
		"SELECT COUNT(*) FROM ip_leases":                                                                       1,
		"SELECT COUNT(*) FROM diameter_peers WHERE role = 'smsc'":                                              1,
		"SELECT COUNT(*) FROM sms_smsc_service_centres":                                                        1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_subscribers_msisdn'":          1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_diameter_peers_identity'":     1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_subscribers_roaming_partner'": 1,
		"SELECT COUNT(*) FROM pragma_foreign_key_check":                                                        0,
	} {
		if got := countRows(t, conn, query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, msisdn)
		VALUES ('s3', '001010000000003', '000000000003', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', '15550001')`)
	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('dup', 'roaming', 'SMSC.example.org', '10.0.0.6', 3868)`)

	mustExec(t, conn, "DELETE FROM diameter_peers WHERE id = 'smsc-1'")

	if got := countRows(t, conn, "SELECT COUNT(*) FROM sms_smsc_service_centres"); got != 0 {
		t.Errorf("service centres after SMSC peer delete = %d, want 0", got)
	}

	mustExec(t, conn, "DELETE FROM subscribers WHERE id = 's1'")

	for _, table := range []string{"daily_usage", "ue_registrations", "ip_leases"} {
		if got := countRows(t, conn, "SELECT COUNT(*) FROM "+table); got != 0 {
			t.Errorf("%s after subscriber delete = %d, want 0", table, got)
		}
	}
}

func TestMigrationV26EnforcesRoamingConstraints(t *testing.T) {
	conn := openV25WithData(t)

	mustExec(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('hss', 'roaming', 'hss.example.org', '10.0.0.7', 3868)`)
	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('x', 'hss', 'x.example.org', '10.0.0.8', 3868)`)

	mustExec(t, conn, `INSERT INTO roaming_partners (id, name, profileID, diameterPeerID) VALUES ('rp', 'partner', 'pr', 'hss')`)
	mustExec(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp', '208', '93'), ('rp', '208', '94')`)
	mustExec(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM, nrfAPIRoot)
		VALUES ('rp5g', 'partner-5g', 'pr', 'sepp1.5gc.mnc410.mcc310.3gppnetwork.org', 'https://sepp.example.org', 'pem', 'http://nrf.example.org')`)
	mustExec(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '310', '410')`)

	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID) VALUES ('a', 'partner', 'pr')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID) VALUES ('e', 'no-profile', 'missing')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN) VALUES ('f', 'partial-5g', 'pr', 'sepp1.example.org')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, nrfAPIRoot) VALUES ('g', 'nrf-only', 'pr', 'http://nrf.example.org')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM) VALUES ('h', 'empty', 'pr', '', '', '')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM) VALUES ('i', 'upper', 'pr', 'SEPP1.example.org', 'https://x', 'pem')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM) VALUES ('j', 'same-sepp', 'pr', 'sepp1.5gc.mnc410.mcc310.3gppnetwork.org', 'https://x', 'pem')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '208', '93')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '20', '95')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '208', '9')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('missing', '208', '96')`)

	mustExec(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, roamingPartnerID) VALUES ('v1', '208930000000001', '', '', '', 'pr', 'rp')`)
	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, roamingPartnerID)
		VALUES ('v2', '208930000000002', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', 'rp')`)
	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, msisdn, roamingPartnerID) VALUES ('v3', '208930000000003', '', '', '', 'pr', '15550009', 'rp')`)
	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID) VALUES ('h1', '001010000000009', '', '', '', 'pr')`)
	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, roamingPartnerID) VALUES ('v4', '208930000000004', '', '', '', 'pr', 'missing')`)

	mustExec(t, conn, `INSERT INTO ue_registrations (imsi, type, nodeID, purged, version) VALUES ('208930000000001', 'mme', 'node-1', 0, 2)`)
	mustExec(t, conn, `INSERT INTO roaming_daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20000, '208930000000001', '208', '93', 1, 2)`)
	mustExec(t, conn, `INSERT INTO roaming_daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20000, '999990000000001', '999', '99', 1, 2)`)
	mustFail(t, conn, `INSERT INTO roaming_daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20000, '208930000000002', '310', '410', 1, 2)`)
	mustFail(t, conn, `INSERT INTO roaming_daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20001, '208930000000001', '208', '93', -1, 2)`)

	mustFail(t, conn, "DELETE FROM diameter_peers WHERE id = 'hss'")
	mustFail(t, conn, "DELETE FROM profiles WHERE id = 'pr'")
	mustFail(t, conn, "DELETE FROM roaming_partners WHERE id = 'rp'")

	mustExec(t, conn, "DELETE FROM subscribers WHERE id = 'v1'")
	mustExec(t, conn, "DELETE FROM roaming_partners WHERE id = 'rp'")

	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM subscribers WHERE roamingPartnerID IS NOT NULL":  0,
		"SELECT COUNT(*) FROM subscribers":                                     2,
		"SELECT COUNT(*) FROM ue_registrations WHERE imsi = '208930000000001'": 0,
		"SELECT COUNT(*) FROM roaming_partner_plmns WHERE partnerID = 'rp'":    0,
		"SELECT COUNT(*) FROM roaming_daily_usage":                             2,
	} {
		if got := countRows(t, conn, query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	mustExec(t, conn, "DELETE FROM diameter_peers WHERE id = 'hss'")

	mustFail(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (1, 'SEPP1.example.org', 'generated', 'chain', 'key')`)
	mustFail(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (0, 'sepp0.example.org', 'generated', 'chain', 'key')`)
	mustFail(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (64, 'sepp64.example.org', 'generated', 'chain', 'key')`)
	mustFail(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (1, 'sepp1.example.org', 'other', 'chain', 'key')`)
	mustExec(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (1, 'sepp1.example.org', 'generated', 'chain', 'key')`)
	mustExec(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (2, 'sepp2.example.org', 'imported', 'chain', 'key')`)
	mustFail(t, conn, `INSERT INTO sepp_identities (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (3, 'sepp1.example.org', 'imported', 'chain', 'key')`)
	mustFail(t, conn, `INSERT INTO sepp_ca (certPEM, keyPEM) VALUES ('', 'key')`)
	mustExec(t, conn, `INSERT INTO sepp_ca (certPEM, keyPEM) VALUES ('cert', 'key')`)
	mustFail(t, conn, `INSERT INTO sepp_ca (certPEM, keyPEM) VALUES ('cert2', 'key2')`)
}
