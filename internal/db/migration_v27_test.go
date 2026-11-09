// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

func openV26WithData(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	if err := runMigrations(ctx, conn, 26); err != nil {
		t.Fatalf("migrate to v26: %v", err)
	}

	seed := []string{
		`INSERT INTO profiles (id, name, ueAmbrUplink, ueAmbrDownlink, allow4G, allow5G) VALUES ('pr', 'default', '1 Gbps', '1 Gbps', 1, 1)`,
		`INSERT INTO data_networks (id, name, ipPool, ipv6Pool, dns, mtu) VALUES ('dn', 'internet', '10.45.0.0/16', '', '8.8.8.8', 1400)`,
		`INSERT INTO network_slices (id, sst, sd, name) VALUES ('sl', 1, NULL, 'default')`,
		`INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink) VALUES ('po', 'default', 'pr', 'sl', 'dn', 9, 1, '200 Mbps', '200 Mbps')`,
		`INSERT INTO slice_data_networks (sliceID, dataNetworkID, nat, localSwitch, networkInstance) VALUES ('sl', 'dn', 0, 1, 'default')`,
		`INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn)
		   VALUES ('s1', '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', 'first', '15550001')`,
		`INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn)
		   VALUES ('s2', '001010000000002', '000000000002', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', '', '')`,
		`INSERT INTO daily_usage (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES (20000, '001010000000001', 10, 20)`,
		`INSERT INTO ue_registrations (imsi, type, nodeID, purged, version) VALUES ('001010000000001', 'mme', 'node-1', 0, 1)`,
		`INSERT INTO ip_leases (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType) VALUES ('l1', 'dn', X'0A2D0001', '001010000000001', 5, 'dynamic', 1, 'node-1', 'ipv4')`,
		`INSERT INTO flow_reports (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action)
		   VALUES ('001010000000001', '10.45.0.1', '8.8.8.8', 1000, 53, 17, 1, 80, 1, 2, 'uplink', 0)`,
		`INSERT INTO sms_smscs (id, name) VALUES ('smsc-1', 'smsc.example.org')`,
		`INSERT INTO diameter_peers (id, role, diameterIdentity, address, port, smscId) VALUES ('smsc-1', 'smsc', 'smsc.example.org', '10.0.0.5', 3868, 'smsc-1')`,
		`INSERT INTO diameter_peers (id, role, diameterIdentity, address, port, smscId) VALUES ('smsc-1b', 'smsc', 'smsc-b.example.org', '10.0.0.9', 3868, 'smsc-1')`,
		`INSERT INTO sms_smsc_service_centres (serviceCentre, smscId) VALUES ('15550000000', 'smsc-1')`,
	}

	for _, stmt := range seed {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := runMigrations(ctx, conn, 27); err != nil {
		t.Fatalf("migrate to v27: %v", err)
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

func TestMigrationV27PreservesExistingData(t *testing.T) {
	ctx := context.Background()
	conn := openV26WithData(t)

	var imsi, description, msisdn string

	if err := conn.QueryRowContext(ctx, "SELECT imsi, description, msisdn FROM subscribers WHERE id = 's1'").Scan(&imsi, &description, &msisdn); err != nil {
		t.Fatalf("read subscriber: %v", err)
	}

	if imsi != "001010000000001" || description != "first" || msisdn != "15550001" {
		t.Fatalf("subscriber after v27 = %s/%s/%s", imsi, description, msisdn)
	}

	for _, table := range []string{"subscribers", "ue_registrations", "ip_leases", "flow_reports"} {
		var fks []string

		rows, err := conn.QueryContext(ctx, "SELECT \"table\" FROM pragma_foreign_key_list('"+table+"')")
		if err != nil {
			t.Fatalf("foreign keys of %s: %v", table, err)
		}

		for rows.Next() {
			var parent string
			if err := rows.Scan(&parent); err != nil {
				t.Fatalf("scan: %v", err)
			}

			fks = append(fks, parent)
		}

		_ = rows.Close()

		for _, parent := range fks {
			if parent == "subscribers" && table != "subscribers" {
				t.Errorf("%s still references subscribers", table)
			}
		}
	}

	var subscribersSQL string

	if err := conn.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE name = 'subscribers'").Scan(&subscribersSQL); err != nil {
		t.Fatalf("read subscribers schema: %v", err)
	}

	if strings.Contains(subscribersSQL, "roamingPartnerID") {
		t.Errorf("subscribers was rebuilt for roaming: %s", subscribersSQL)
	}

	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM subscribers":                                              2,
		"SELECT COUNT(*) FROM daily_usage":                                              1,
		"SELECT COUNT(*) FROM ue_registrations":                                         1,
		"SELECT COUNT(*) FROM ip_leases":                                                1,
		"SELECT COUNT(*) FROM flow_reports":                                             1,
		"SELECT COUNT(*) FROM diameter_peers WHERE role = 'smsc' AND smscId = 'smsc-1'": 2,
		"SELECT COUNT(*) FROM slice_data_networks WHERE sliceID = 'sl' AND dataNetworkID = 'dn' AND nat = 0 AND localSwitch = 1":                                     1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_slice_data_networks_data_network'":                                                  1,
		"SELECT COUNT(*) FROM sms_smscs WHERE id = 'smsc-1' AND name = 'smsc.example.org'":                                                                           1,
		"SELECT COUNT(*) FROM sms_smsc_service_centres WHERE smscId = 'smsc-1'":                                                                                      1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name IN ('idx_diameter_peers_smsc', 'idx_sms_smscs_name', 'idx_sms_smsc_service_centres_smsc')": 3,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_ue_registrations_version'":                                                          1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_daily_usage_imsi'":                                                                  0,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_subscribers_msisdn'":                                                                1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_diameter_peers_identity'":                                                           1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name IN ('idx_leases_pool', 'idx_leases_imsi', 'idx_leases_session', 'idx_leases_node')":        4,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_ue_registrations_node'":                                                             1,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name LIKE 'idx_flow_reports_%'":                                                                 5,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_visitors_partner'":                                                                  1,
		"SELECT COUNT(*) FROM pragma_foreign_key_check":                                                                                                              0,
	} {
		if got := countRows(t, conn, query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID, msisdn)
		VALUES ('s3', '001010000000003', '000000000003', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 'pr', '15550001')`)
	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('dup', 'roaming', 'SMSC.example.org', '10.0.0.6', 3868)`)

	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('node-2', 'smsc', 'smsc2.example.org', '10.0.0.6', 3868)`)
	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port, smscId) VALUES ('hss', 'roaming', 'hss.example.org', '10.0.0.7', 3868, 'smsc-1')`)
	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port, smscId) VALUES ('node-3', 'smsc', 'smsc3.example.org', '10.0.0.8', 3868, 'missing')`)
	mustExec(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port, smscId) VALUES ('node-2', 'smsc', 'smsc2.example.org', '10.0.0.6', 3868, 'smsc-1')`)
	mustExec(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('hss', 'roaming', 'hss.example.org', '10.0.0.7', 3868)`)

	mustExec(t, conn, "DELETE FROM sms_smscs WHERE id = 'smsc-1'")

	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM sms_smsc_service_centres":           0,
		"SELECT COUNT(*) FROM diameter_peers WHERE role = 'smsc'": 0,
		"SELECT COUNT(*) FROM diameter_peers WHERE id = 'hss'":    1,
		"SELECT COUNT(*) FROM pragma_foreign_key_check":           0,
	} {
		if got := countRows(t, conn, query); got != want {
			t.Errorf("after SMSC delete: %s = %d, want %d", query, got, want)
		}
	}

	var epochDay, uplink, downlink int64

	var mcc, mnc sql.NullString

	if err := conn.QueryRowContext(ctx, "SELECT epochDay, mcc, mnc, bytesUplink, bytesDownlink FROM daily_usage WHERE imsi = '001010000000001'").Scan(&epochDay, &mcc, &mnc, &uplink, &downlink); err != nil {
		t.Fatalf("read daily usage: %v", err)
	}

	if epochDay != 20000 || mcc.Valid || mnc.Valid || uplink != 10 || downlink != 20 {
		t.Fatalf("daily usage after v27 = %d/%v/%v/%d/%d", epochDay, mcc, mnc, uplink, downlink)
	}

	mustExec(t, conn, "DELETE FROM subscribers WHERE id = 's1'")

	for _, table := range []string{"ue_registrations", "ip_leases", "flow_reports"} {
		if got := countRows(t, conn, "SELECT COUNT(*) FROM "+table); got != 1 {
			t.Errorf("%s after a raw subscriber delete = %d, want 1: the cascade is done by the apply, not by SQLite", table, got)
		}
	}

	if got := countRows(t, conn, "SELECT COUNT(*) FROM daily_usage"); got != 1 {
		t.Errorf("daily_usage after subscriber delete = %d, want 1", got)
	}
}

func TestMigrationV27EnforcesRoamingConstraints(t *testing.T) {
	conn := openV26WithData(t)

	mustExec(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('hss', 'roaming', 'hss.example.org', '10.0.0.7', 3868)`)
	mustFail(t, conn, `INSERT INTO diameter_peers (id, role, diameterIdentity, address, port) VALUES ('x', 'hss', 'x.example.org', '10.0.0.8', 3868)`)

	mustExec(t, conn, `INSERT INTO roaming_partners (id, name, profileID, diameterPeerID) VALUES ('rp', 'partner', 'pr', 'hss')`)
	mustExec(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp', '208', '93'), ('rp', '208', '94')`)
	mustExec(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM)
		VALUES ('rp5g', 'partner-5g', 'pr', 'sepp1.5gc.mnc410.mcc310.3gppnetwork.org', 'https://sepp.example.org', 'pem')`)
	mustExec(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc, nrfAPIRoot) VALUES ('rp5g', '310', '410', 'http://nrf.example.org')`)

	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID) VALUES ('a', 'partner', 'pr')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID) VALUES ('e', 'no-profile', 'missing')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN) VALUES ('f', 'partial-5g', 'pr', 'sepp1.example.org')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppN32fURI) VALUES ('g', 'n32f-only', 'pr', 'https://sepp.example.org')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc, nrfAPIRoot) VALUES ('rp5g', '310', '411', '')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM) VALUES ('h', 'empty', 'pr', '', '', '')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM) VALUES ('i', 'upper', 'pr', 'SEPP1.example.org', 'https://x', 'pem')`)
	mustFail(t, conn, `INSERT INTO roaming_partners (id, name, profileID, seppFQDN, seppN32cURI, seppTrustAnchorPEM) VALUES ('j', 'same-sepp', 'pr', 'sepp1.5gc.mnc410.mcc310.3gppnetwork.org', 'https://x', 'pem')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '208', '93')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '20', '95')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('rp5g', '208', '9')`)
	mustFail(t, conn, `INSERT INTO roaming_partner_plmns (partnerID, mcc, mnc) VALUES ('missing', '208', '96')`)

	mustExec(t, conn, `INSERT INTO visitors (imsi, partnerID, msisdn, since) VALUES ('208930000000001', 'rp', '', 1)`)
	mustExec(t, conn, `INSERT INTO visitors (imsi, partnerID, msisdn, since) VALUES ('208930000000002', 'rp', '15550001', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID, since) VALUES ('208930000000001', 'rp', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID, since) VALUES ('208930000000003', 'missing', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID, since) VALUES ('2089', 'rp', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID, since) VALUES ('20893000000000a', 'rp', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID, msisdn, since) VALUES ('208930000000004', 'rp', '+15550001', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID, msisdn, since) VALUES ('208930000000005', 'rp', '1234567890123456', 1)`)
	mustFail(t, conn, `INSERT INTO visitors (imsi, partnerID) VALUES ('208930000000006', 'rp')`)
	mustFail(t, conn, `INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID) VALUES ('h1', '001010000000009', '', '', '', 'missing')`)

	mustExec(t, conn, `INSERT INTO ue_registrations (imsi, type, nodeID, purged, version) VALUES ('208930000000001', 'mme', 'node-1', 0, 2)`)
	mustExec(t, conn, `INSERT INTO ip_leases (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType) VALUES ('l2', 'dn', X'0A2D0002', '208930000000001', 6, 'dynamic', 1, 'node-1', 'ipv4')`)
	mustFail(t, conn, `INSERT INTO ip_leases (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType) VALUES ('l3', 'missing', X'0A2D0003', '208930000000001', 7, 'dynamic', 1, 'node-1', 'ipv4')`)
	mustFail(t, conn, `INSERT INTO ue_registrations (imsi, type, nodeID, purged, version) VALUES ('12345', 'mme', 'node-1', 0, 3)`)
	mustFail(t, conn, `INSERT INTO ue_registrations (imsi, type, nodeID, purged, version) VALUES ('208930000-00001', 'mme', 'node-1', 0, 3)`)
	mustFail(t, conn, `INSERT INTO ip_leases (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType) VALUES ('l4', 'dn', X'0A2D0004', '2089300000 0001', 8, 'dynamic', 1, 'node-1', 'ipv4')`)
	mustExec(t, conn, `INSERT INTO daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20000, '208930000000001', '208', '93', 1, 2)`)
	mustExec(t, conn, `INSERT INTO daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20000, '999990000000001', '999', '99', 1, 2)`)
	mustExec(t, conn, `INSERT INTO daily_usage (epochDay, imsi, bytesUplink, bytesDownlink) VALUES (20000, '001019999999999', 1, 2)`)
	mustFail(t, conn, `INSERT INTO daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20000, '208930000000002', '310', '410', 1, 2)`)
	mustFail(t, conn, `INSERT INTO daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20001, '208930000000001', '208', '93', -1, 2)`)
	mustFail(t, conn, `INSERT INTO daily_usage (epochDay, imsi, mcc, bytesUplink, bytesDownlink) VALUES (20001, '208930000000001', '208', 1, 2)`)
	mustFail(t, conn, `INSERT INTO daily_usage (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (20001, '208930000000001', '208', '9', 1, 2)`)
	mustFail(t, conn, `INSERT INTO daily_usage (epochDay, imsi, bytesUplink, bytesDownlink) VALUES (20001, '12345', 1, 2)`)
	mustFail(t, conn, `INSERT INTO daily_usage (epochDay, imsi, bytesUplink, bytesDownlink) VALUES (20001, '00101999999999x', 1, 2)`)

	mustFail(t, conn, "DELETE FROM diameter_peers WHERE id = 'hss'")
	mustFail(t, conn, "DELETE FROM profiles WHERE id = 'pr'")
	mustFail(t, conn, "DELETE FROM roaming_partners WHERE id = 'rp'")

	mustExec(t, conn, "DELETE FROM visitors WHERE partnerID = 'rp'")
	mustExec(t, conn, "DELETE FROM roaming_partners WHERE id = 'rp'")

	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM visitors":                                     0,
		"SELECT COUNT(*) FROM subscribers":                                  2,
		"SELECT COUNT(*) FROM roaming_partner_plmns WHERE partnerID = 'rp'": 0,
		"SELECT COUNT(*) FROM daily_usage":                                  4,
		"SELECT COUNT(*) FROM pragma_foreign_key_check":                     0,
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

func TestMigrationV27KeepsSMSCNodesApartFromRoamingPeers(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 26)

	smsc := SMSC{
		ID:             "01890000-0000-7000-8000-0000000000aa",
		Name:           "smsc",
		ServiceCentres: []string{"15550000009"},
		Nodes: []SMSCNode{
			{ID: "01890000-0000-7000-8000-0000000000ab", DiameterIdentity: "smsc1.example.org", Address: "192.0.2.10", Port: 3868},
			{ID: "01890000-0000-7000-8000-0000000000ac", DiameterIdentity: "smsc2.example.org", Address: "192.0.2.11", Port: 3868},
		},
	}

	if err := d.CreateSMSC(ctx, &smsc); err != nil {
		t.Fatalf("CreateSMSC at v26: %v", err)
	}

	if err := d.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "hss", DiameterIdentity: "hss.example.org", Address: "10.0.0.7", Port: 3868}); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("CreateRoamingDiameterPeer at v26 = %v, want ErrMigrationPending", err)
	}

	migrateTestDatabase(t, d, 27)

	assertSMSCs(t, d, "at v27", []SMSC{smsc})

	if roaming, err := d.ListRoamingDiameterPeers(ctx); err != nil || len(roaming) != 0 {
		t.Fatalf("roaming Diameter peers after v27 = %+v, %v, want none: SMSC nodes are not roaming peers", roaming, err)
	}

	if err := d.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "dup", DiameterIdentity: "SMSC1.example.org", Address: "10.0.0.8", Port: 3868}); err == nil {
		t.Fatal("a roaming peer took the Diameter identity of an SMSC node")
	}

	if err := d.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "hss", DiameterIdentity: "hss.example.org", Address: "10.0.0.7", Port: 3868}); err != nil {
		t.Fatalf("CreateRoamingDiameterPeer at v27: %v", err)
	}

	if _, err := d.applyDeleteSMSC(ctx, &stringPayload{Value: smsc.ID}); err != nil {
		t.Fatalf("delete the migrated SMSC: %v", err)
	}

	if roaming, err := d.ListRoamingDiameterPeers(ctx); err != nil || len(roaming) != 1 || roaming[0].ID != "hss" {
		t.Fatalf("roaming Diameter peers after the SMSC delete = %+v, %v, want hss", roaming, err)
	}
}

func TestWithheldLeaseOwnersCoverVisitors(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 27)
	f := setupForwardingFixture(t, d)

	profile, err := d.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	if err := d.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "peer", DiameterIdentity: "dea.example.org", Address: "10.0.0.7", Port: 3868}); err != nil {
		t.Fatalf("create roaming diameter peer: %v", err)
	}

	peer := "peer"
	if err := d.CreateRoamingPartner(ctx, &RoamingPartner{ID: "partner", Name: "partner", PLMNs: []RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileID: profile.ID, DiameterPeerID: &peer}); err != nil {
		t.Fatalf("create roaming partner: %v", err)
	}

	const imsi = "208930000000002"

	if _, err := d.RegisterVisitorUE(ctx, imsi, "partner", UERegistrationTypeMME, "1", ""); err != nil {
		t.Fatalf("register visitor: %v", err)
	}

	if _, err := d.AllocateIPLease(ctx, f.internet, "ipv4", imsi, 1, "1"); err != nil {
		t.Fatalf("allocate a lease: %v", err)
	}

	got, err := d.WithheldLeaseOwners(ctx, "1")
	if err != nil {
		t.Fatalf("WithheldLeaseOwners: %v", err)
	}

	if want := map[string]LeaseWithholding{imsi + "/" + f.internet: {IPv4: true}}; !maps.Equal(got, want) {
		t.Fatalf("withheld lease owners %v, want %v", got, want)
	}
}
