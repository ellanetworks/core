// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/supportbundle"
	"github.com/google/uuid"
)

func TestExportSupportData_Default(t *testing.T) {
	database := setupTestDB(t)

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{IncludeAuditLogs: true, IncludeSubscriberUsage: true})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	bm, ok := out["bundle_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("bundle_metadata missing or wrong type")
	}

	if bm["version"] != "2.0" {
		t.Fatalf("unexpected bundle metadata version: %v", bm["version"])
	}

	redaction, ok := bm["redaction"].(map[string]any)
	if !ok {
		t.Fatalf("redaction metadata missing or wrong type: %T", bm["redaction"])
	}

	if !reflect.DeepEqual(redaction, map[string]any{"policy": "secrets-only"}) {
		t.Fatalf("unexpected redaction metadata: %#v", redaction)
	}

	if _, err := time.Parse(time.RFC3339, bm["captured_at"].(string)); err != nil {
		t.Fatalf("captured_at not a RFC3339 timestamp: %v", err)
	}

	if opm, ok := out["operator"].(map[string]any); !ok {
		t.Fatalf("operator missing or wrong type: %T", out["operator"])
	} else {
		if opCode, ok := opm["OperatorCode"].(string); !ok || opCode != "*" {
			t.Fatalf("operator OperatorCode not redacted; got %#v", opm["OperatorCode"])
		}

		for _, key := range []string{"SpnFullName", "SpnShortName", "AmfRegionID", "AmfSetID", "ClusterID", "Ciphering", "Integrity"} {
			if _, ok := opm[key]; !ok {
				t.Fatalf("operator export missing %s", key)
			}
		}
	}

	if hnKeys, ok := out["home_network_keys"].([]map[string]any); !ok {
		t.Fatalf("home_network_keys missing or wrong type: %T", out["home_network_keys"])
	} else if len(hnKeys) == 0 {
		t.Fatalf("expected at least one home network key")
	} else {
		km := hnKeys[0]
		if pk, ok := km["PrivateKey"].(string); !ok || pk != "*" {
			t.Fatalf("home network key PrivateKey not redacted; got %#v", km["PrivateKey"])
		}
	}

	if policies, ok := out["policies"].([]db.Policy); !ok {
		t.Fatalf("policies missing or wrong type: %T", out["policies"])
	} else if len(policies) == 0 {
		t.Fatalf("expected at least one policy in export")
	}

	if dns, ok := out["networking"].([]db.DataNetwork); !ok {
		t.Fatalf("networking missing or wrong type: %T", out["networking"])
	} else if len(dns) == 0 {
		t.Fatalf("expected at least one data network in export")
	}

	if _, ok := out["subscribers"].([]any); !ok {
		t.Fatalf("subscribers missing or wrong type: %T", out["subscribers"])
	}

	sections := bm["sections"].(map[string]supportbundle.SectionStatus)

	for _, key := range []string{
		"operator", "home_network_keys", "policies", "networking", "subscribers", "ip_leases",
		"profiles", "network_slices", "network_rules", "cell_positions", "cluster_members", "routes",
		"nat_settings", "n3_settings", "flow_accounting_settings", "local_switch_settings", "sms_settings",
		"diameter_peers", "pcscf_addresses", "retention_policies", "bgp_settings", "bgp_peers", "bgp_import_prefixes",
		"ims_application_servers", "ims_filter_criteria",
		"ue_registrations", "ims_registrations", "sms_message_waiting", "subscriber_framed_routes",
		"usage_per_day", "usage_per_subscriber", "audit_logs", "radio_logs",
	} {
		if _, ok := out[key]; !ok {
			t.Fatalf("missing %s section in export", key)
		}

		if sections[key].Status != "complete" {
			t.Fatalf("unexpected %s section status: %#v", key, sections[key])
		}
	}
}

func TestExportSupportData_WithEntries(t *testing.T) {
	database := setupTestDB(t)

	dn := &db.DataNetwork{Name: "support-net", IPv4Pool: "10.99.0.0/24", DNS: "1.1.1.1", MTU: 1400}
	if err := database.CreateDataNetwork(context.Background(), dn); err != nil {
		t.Fatalf("CreateDataNetwork failed: %v", err)
	}

	createdDN, err := database.GetDataNetwork(context.Background(), dn.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork failed: %v", err)
	}

	defaultProfile, err := database.GetProfile(context.Background(), db.InitialProfileName)
	if err != nil {
		t.Fatalf("Couldn't get default profile: %v", err)
	}

	defaultSlice, err := database.GetNetworkSlice(context.Background(), db.InitialSliceName)
	if err != nil {
		t.Fatalf("Couldn't get default slice: %v", err)
	}

	policy := &db.Policy{Name: "support-policy", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkID: createdDN.ID, ProfileID: defaultProfile.ID, SliceID: defaultSlice.ID}
	if err := database.CreatePolicy(context.Background(), policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	sub := &db.Subscriber{
		Imsi:           "001010000000001",
		Msisdn:         "15551234567",
		SequenceNumber: "000000000001",
		PermanentKey:   strings.Repeat("p", 32),
		Opc:            strings.Repeat("o", 32),
		ProfileID:      defaultProfile.ID,
	}
	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{IncludeAuditLogs: true})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	subsAny, ok := out["subscribers"].([]any)
	if !ok {
		t.Fatalf("subscribers missing or wrong type: %T", out["subscribers"])
	}

	if len(subsAny) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subsAny))
	}

	subEntry, ok := subsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("subscriber entry wrong type: %T", subsAny[0])
	}

	if !reflect.DeepEqual(subEntry, map[string]any{"imsi": sub.Imsi, "msisdn": sub.Msisdn, "profile_id": defaultProfile.ID}) {
		t.Fatalf("subscriber identities changed or non-allowlisted fields exported: %#v", subEntry)
	}

	policies, ok := out["policies"].([]db.Policy)
	if !ok {
		t.Fatalf("policies missing or wrong type: %T", out["policies"])
	}

	foundPolicy := false

	for _, p := range policies {
		if p.Name == policy.Name && p.DataNetworkID == createdDN.ID {
			foundPolicy = true
		}
	}

	if !foundPolicy {
		t.Fatalf("created policy not found in export")
	}

	networks, ok := out["networking"].([]db.DataNetwork)
	if !ok {
		t.Fatalf("networking missing or wrong type: %T", out["networking"])
	}

	foundNet := false

	for _, n := range networks {
		if n.Name == dn.Name && n.IPv4Pool == dn.IPv4Pool {
			foundNet = true
		}
	}

	if !foundNet {
		t.Fatalf("created data network not found in export")
	}
}

func TestExportSupportData_IMSServices(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	as := &db.IMSApplicationServer{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}
	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("CreateIMSApplicationServer failed: %v", err)
	}

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	register := "REGISTER"
	handling := db.IMSDefaultHandlingSessionContinued
	trigger := &db.IMSTrigger{ConditionsMatch: db.IMSConditionsMatchAll, Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{{Method: &register}}}}}

	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, []db.IMSInitialFilterCriterion{
		{ApplicationServerID: as.ID, Trigger: trigger, DefaultHandling: &handling, IncludeRegisterRequest: true},
		{ApplicationServerID: as.ID},
	}); err != nil {
		t.Fatalf("SetProfileIMSFilterCriteria failed: %v", err)
	}

	out, err := database.ExportSupportData(ctx, supportbundle.ExportOptions{})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	wantServers := []any{map[string]any{"name": "mcptt", "server_name": "sip:mcptt.ims.example.org", "public_service_identities": []string{"sip:!.*!@mcptt.ims.example.org"}}}
	if !reflect.DeepEqual(out["ims_application_servers"], wantServers) {
		t.Fatalf("ims_application_servers = %#v, want %#v", out["ims_application_servers"], wantServers)
	}

	criteria, ok := out["ims_filter_criteria"].([]any)
	if !ok || len(criteria) != 2 {
		t.Fatalf("ims_filter_criteria = %#v, want 2 entries", out["ims_filter_criteria"])
	}

	first := criteria[0].(map[string]any)
	if first["profile"] != db.InitialProfileName || first["priority"] != 0 || first["application_server_name"] != "mcptt" ||
		!reflect.DeepEqual(first["trigger"], trigger) || first["include_register_request"] != true {
		t.Fatalf("first filter criterion = %#v", first)
	}

	if second := criteria[1].(map[string]any); second["priority"] != 1 || second["trigger"] != (*db.IMSTrigger)(nil) {
		t.Fatalf("second filter criterion = %#v", second)
	}

	sections := out["bundle_metadata"].(map[string]any)["sections"].(map[string]supportbundle.SectionStatus)
	for key, count := range map[string]int{"ims_application_servers": 1, "ims_filter_criteria": 2} {
		if sections[key].Status != "complete" || sections[key].Count != count {
			t.Fatalf("%s section = %#v, want complete with %d", key, sections[key], count)
		}
	}
}

func TestListAllSMSWaitingGroupsCentres(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatal(err)
	}

	for _, imsi := range []string{"001010000000001", "001010000000002"} {
		if err := database.CreateSubscriber(ctx, &db.Subscriber{Imsi: imsi, ProfileID: profile.ID, PermanentKey: strings.Repeat("p", 32), Opc: strings.Repeat("o", 32), SequenceNumber: "000000000001"}); err != nil {
			t.Fatal(err)
		}
	}

	for _, update := range []db.SMSWaitingUpdate{
		{IMSI: "001010000000001", ServiceCentre: "+15550000002"},
		{IMSI: "001010000000001", ServiceCentre: "+15550000001"},
		{IMSI: "001010000000002", ServiceCentre: "+15550000003", MemoryFull: true},
	} {
		if err := database.RecordSMSWaiting(ctx, update); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := database.ListAllSMSWaiting(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 || !reflect.DeepEqual(rows[0].ServiceCentres, []string{"+15550000001", "+15550000002"}) || !reflect.DeepEqual(rows[1].ServiceCentres, []string{"+15550000003"}) {
		t.Fatalf("incorrect grouped centres: %#v", rows)
	}
}

func TestSupportCollectionFailureVisible(t *testing.T) {
	database := setupTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out, err := database.ExportSupportData(ctx, supportbundle.ExportOptions{IncludeSubscriberUsage: true})
	if err != nil {
		t.Fatal(err)
	}

	sections := out["bundle_metadata"].(map[string]any)["sections"].(map[string]supportbundle.SectionStatus)
	if sections["operator"].Status != "failed" || sections["subscribers"].Status != "failed" || sections["network_rules"].Status != "failed" || sections["network_rules"].Reason != "policy collection incomplete" {
		t.Fatalf("failed collectors reported success: %#v", sections)
	}

	for _, section := range []string{"usage_per_day", "usage_per_subscriber"} {
		if sections[section].Status != "failed" {
			t.Fatalf("failed %s collector reported success: %#v", section, sections[section])
		}
	}
}

func TestSupportRetentionCount(t *testing.T) {
	database := setupTestDB(t)

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}

	sections := out["bundle_metadata"].(map[string]any)["sections"].(map[string]supportbundle.SectionStatus)
	if sections["retention_policies"].Count != 4 || sections["retention_policies"].Status != "complete" {
		t.Fatalf("wrong retention count: %#v", sections["retention_policies"])
	}
}

func TestExportSupportData_IdentitySectionsPreserved(t *testing.T) {
	database := setupTestDB(t)

	defaultProfile, err := database.GetProfile(context.Background(), db.InitialProfileName)
	if err != nil {
		t.Fatalf("Couldn't get default profile: %v", err)
	}

	imsi := "001010000000001"
	actor := "admin@example.com"

	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   strings.Repeat("p", 32),
		Opc:            strings.Repeat("o", 32),
		ProfileID:      defaultProfile.ID,
	}
	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}

	if _, err := database.RegisterUE(context.Background(), imsi, db.UERegistrationTypeAMF3GPPAccess, "node-1", ""); err != nil {
		t.Fatalf("RegisterUE failed: %v", err)
	}

	if _, _, err := database.CompareAndSwapIMSRegistration(context.Background(), imsi, nil, &db.IMSRegistration{
		State:           db.IMSRegistered,
		ServerName:      "scscf.ims.test",
		OriginHost:      "h",
		OriginRealm:     "r",
		PrivateIdentity: imsi + "@ims.test",
		ProfileDigest:   "digest",
	}); err != nil {
		t.Fatalf("CompareAndSwapIMSRegistration failed: %v", err)
	}

	if err := database.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{
		IMSI:          imsi,
		ServiceCentre: "+15550001111",
		MemoryFull:    true,
	}); err != nil {
		t.Fatalf("RecordSMSWaiting failed: %v", err)
	}

	today := time.Now().UTC().Unix() / 86400
	if err := database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      today,
		IMSI:          imsi,
		BytesUplink:   100,
		BytesDownlink: 200,
	}); err != nil {
		t.Fatalf("IncrementDailyUsage failed: %v", err)
	}

	auditID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}

	al := &dbwriter.AuditLog{
		ID:        auditID.String(),
		Timestamp: dbwriter.EpochMillis(time.Now().UTC()),
		Level:     "INFO",
		Actor:     actor,
		Action:    "test-action",
		IP:        "127.0.0.1",
		Details:   "subscriber imsi-" + imsi + " registered by " + actor + "\n  MSISDN=+15551234567",
	}
	if err := database.InsertAuditLog(context.Background(), al); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{IncludeAuditLogs: true, IncludeSubscriberUsage: true})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	regsAny, ok := out["ue_registrations"].([]any)
	if !ok || len(regsAny) != 1 {
		t.Fatalf("ue_registrations missing or wrong length: %T %#v", out["ue_registrations"], out["ue_registrations"])
	}

	reg := regsAny[0].(map[string]any)
	if reg["imsi"] != imsi {
		t.Fatalf("ue_registrations imsi changed: %#v", reg["imsi"])
	}

	if reg["node_id"] != "node-1" {
		t.Fatalf("unexpected ue_registrations node_id: %#v", reg["node_id"])
	}

	imsRegsAny, ok := out["ims_registrations"].([]any)
	if !ok || len(imsRegsAny) != 1 {
		t.Fatalf("ims_registrations missing or wrong length: %T %#v", out["ims_registrations"], out["ims_registrations"])
	}

	imsReg := imsRegsAny[0].(map[string]any)
	if imsReg["imsi"] != imsi {
		t.Fatalf("ims_registrations imsi changed: %#v", imsReg["imsi"])
	}

	if imsReg["server_name"] != "scscf.ims.test" {
		t.Fatalf("unexpected ims_registrations server_name: %#v", imsReg["server_name"])
	}

	waitingAny, ok := out["sms_message_waiting"].([]any)
	if !ok || len(waitingAny) != 1 {
		t.Fatalf("sms_message_waiting missing or wrong length: %T %#v", out["sms_message_waiting"], out["sms_message_waiting"])
	}

	waiting := waitingAny[0].(map[string]any)
	if waiting["imsi"] != imsi {
		t.Fatalf("sms_message_waiting imsi changed: %#v", waiting["imsi"])
	}

	if !reflect.DeepEqual(waiting["service_centres"], []string{"+15550001111"}) {
		t.Fatalf("sms_message_waiting service centres changed: %#v", waiting["service_centres"])
	}

	if waiting["memory_full"] != true {
		t.Fatalf("unexpected sms_message_waiting memory_full: %#v", waiting["memory_full"])
	}

	usageAny, ok := out["usage_per_subscriber"].([]any)
	if !ok || len(usageAny) != 1 {
		t.Fatalf("usage_per_subscriber missing or wrong length: %T %#v", out["usage_per_subscriber"], out["usage_per_subscriber"])
	}

	usage := usageAny[0].(map[string]any)
	if usage["imsi"] != imsi {
		t.Fatalf("usage_per_subscriber imsi changed: %#v", usage["imsi"])
	}

	if usage["bytes_uplink"] != int64(100) {
		t.Fatalf("unexpected usage_per_subscriber bytes_uplink: %#v", usage["bytes_uplink"])
	}

	auditAny, ok := out["audit_logs"].([]any)
	if !ok || len(auditAny) != 1 {
		t.Fatalf("audit_logs missing or wrong length: %T %#v", out["audit_logs"], out["audit_logs"])
	}

	audit := auditAny[0].(map[string]any)
	if audit["actor"] != actor {
		t.Fatalf("audit_logs actor changed: %#v", audit["actor"])
	}

	if audit["action"] != "test-action" {
		t.Fatalf("unexpected audit_logs action: %#v", audit["action"])
	}

	if audit["details"] != al.Details {
		t.Fatalf("audit_logs details changed: %#v", audit["details"])
	}
}

func TestExportSupportData_AuditDetailsPreserved(t *testing.T) {
	database := setupTestDB(t)

	actor := "admin@ellanetworks.com"

	entries := []struct {
		action  string
		details string
	}{
		{"create_user", "User created user: newuser@ellanetworks.com with role: 2"},
		{"update_user", "User updated user: renamed@ellanetworks.com"},
		{"update_user_password", "User updated password for user: newuser@ellanetworks.com"},
		{"delete_user", "User deleted user: operator@[127.0.0.1]"},
		{"custom_action", "Unrecognized details: admin@example.com imsi-001010000000001\n  MSISDN=+15551234567"},
	}

	for i, e := range entries {
		auditID, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("uuid.NewV7: %v", err)
		}

		al := &dbwriter.AuditLog{
			ID:        auditID.String(),
			Timestamp: dbwriter.EpochMillis(time.Now().UTC().Add(time.Duration(i) * time.Second)),
			Level:     "INFO",
			Actor:     actor,
			Action:    e.action,
			IP:        "127.0.0.1",
			Details:   e.details,
		}
		if err := database.InsertAuditLog(context.Background(), al); err != nil {
			t.Fatalf("InsertAuditLog failed: %v", err)
		}
	}

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{IncludeAuditLogs: true})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	auditAny, ok := out["audit_logs"].([]any)
	if !ok || len(auditAny) != len(entries) {
		t.Fatalf("audit_logs missing or wrong length: %T %#v", out["audit_logs"], out["audit_logs"])
	}

	for _, e := range entries {
		found := false

		for _, a := range auditAny {
			entry, ok := a.(map[string]any)
			if !ok {
				t.Fatalf("audit entry wrong type: %T", a)
			}

			if entry["action"] != e.action {
				continue
			}

			if entry["actor"] != actor {
				t.Fatalf("audit actor changed: %#v", entry["actor"])
			}

			if entry["details"] != e.details {
				t.Fatalf("audit details changed: got %#v, want %q", entry["details"], e.details)
			}

			found = true
		}

		if !found {
			t.Fatalf("audit entry for action %q not found", e.action)
		}
	}
}

func TestExportSupportData_IndependentUsageAndAuditPermissions(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatal(err)
	}

	imsi := "001010000000001"
	if err := database.CreateSubscriber(ctx, &db.Subscriber{
		Imsi: imsi, ProfileID: profile.ID, PermanentKey: strings.Repeat("p", 32), Opc: strings.Repeat("o", 32), SequenceNumber: "000000000001",
	}); err != nil {
		t.Fatal(err)
	}

	usage := db.DailyUsage{EpochDay: db.DaysSinceEpoch(time.Now().UTC().AddDate(0, 0, -1)), IMSI: imsi, BytesUplink: 100, BytesDownlink: 200}
	if err := database.IncrementDailyUsage(ctx, usage); err != nil {
		t.Fatal(err)
	}

	auditID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}

	al := &dbwriter.AuditLog{
		ID:        auditID.String(),
		Timestamp: dbwriter.EpochMillis(time.Now().UTC()),
		Level:     "INFO",
		Actor:     "admin@ellanetworks.com",
		Action:    "user:create",
		IP:        "127.0.0.1",
		Details:   "User created user: newuser@ellanetworks.com with role: 2",
	}
	if err := database.InsertAuditLog(ctx, al); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	for name, options := range map[string]supportbundle.ExportOptions{
		"neither":    {},
		"audit_only": {IncludeAuditLogs: true},
		"usage_only": {IncludeSubscriberUsage: true},
		"both":       {IncludeAuditLogs: true, IncludeSubscriberUsage: true},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := database.ExportSupportData(ctx, options)
			if err != nil {
				t.Fatalf("ExportSupportData failed: %v", err)
			}

			sections := out["bundle_metadata"].(map[string]any)["sections"].(map[string]supportbundle.SectionStatus)

			for section, included := range map[string]bool{
				"audit_logs":           options.IncludeAuditLogs,
				"usage_per_day":        options.IncludeSubscriberUsage,
				"usage_per_subscriber": options.IncludeSubscriberUsage,
			} {
				wantStatus := supportbundle.SectionStatus{Status: "omitted", Reason: "permission restricted"}
				if included {
					wantStatus = supportbundle.SectionStatus{Status: "complete", Count: 1}
				}

				if _, present := out[section]; present != included {
					t.Errorf("%s present=%t, want %t", section, present, included)
				}

				if sections[section] != wantStatus {
					t.Errorf("unexpected %s status: got %+v, want %+v", section, sections[section], wantStatus)
				}
			}

			if options.IncludeSubscriberUsage {
				for section, want := range map[string][]any{
					"usage_per_day":        {map[string]any{"day": usage.EpochDay, "bytes_uplink": usage.BytesUplink, "bytes_downlink": usage.BytesDownlink}},
					"usage_per_subscriber": {map[string]any{"imsi": imsi, "bytes_uplink": usage.BytesUplink, "bytes_downlink": usage.BytesDownlink}},
				} {
					if !reflect.DeepEqual(out[section], want) {
						t.Errorf("%s changed: got %#v, want %#v", section, out[section], want)
					}
				}
			}

			for _, section := range []string{"subscribers", "radio_logs"} {
				if _, ok := out[section]; !ok || sections[section].Status != "complete" {
					t.Errorf("%s must remain available: %+v", section, sections[section])
				}
			}

			if !options.IncludeAuditLogs {
				marshaled, err := json.Marshal(out)
				if err != nil {
					t.Fatalf("marshal export: %v", err)
				}

				if strings.Contains(string(marshaled), al.Actor) {
					t.Fatal("export leaked audit actor email")
				}
			}
		})
	}
}

func TestExportSupportData_RadioLogsExcludeRaw(t *testing.T) {
	database := setupTestDB(t)

	re := &dbwriter.RadioEvent{
		Timestamp:     dbwriter.EpochMillis(time.Now().UTC()),
		Protocol:      "NGAP",
		MessageType:   "InitialUEMessage",
		Direction:     "inbound",
		LocalAddress:  "10.0.0.1:38412",
		RemoteAddress: "192.0.2.1:38412",
		RadioName:     "radio-admin@example.com",
		Raw:           []byte("raw-packet-with-security-keys"),
		Details:       "registration for imsi-001010000000001\n  MSISDN=+15551234567 admin@example.com",
	}

	if err := database.InsertRadioEvent(context.Background(), re); err != nil {
		t.Fatalf("InsertRadioEvent failed: %v", err)
	}

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{IncludeAuditLogs: true})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	logsAny, ok := out["radio_logs"]
	if !ok {
		t.Fatalf("radio_logs missing from export")
	}

	logsSlice, ok := logsAny.([]any)
	if !ok {
		t.Fatalf("radio_logs wrong type: %T", logsAny)
	}

	if len(logsSlice) != 1 {
		t.Fatalf("expected 1 radio log, got %d", len(logsSlice))
	}

	entry, ok := logsSlice[0].(map[string]any)
	if !ok {
		t.Fatalf("radio_log entry wrong type: %T", logsSlice[0])
	}

	if _, ok := entry["Raw"]; ok {
		t.Fatalf("radio log entry must not contain Raw")
	}

	if _, ok := entry["raw"]; ok {
		t.Fatalf("radio log entry must not contain raw")
	}

	for key, want := range map[string]any{
		"timestamp":      re.Timestamp,
		"protocol":       re.Protocol,
		"message_type":   re.MessageType,
		"direction":      re.Direction,
		"local_address":  re.LocalAddress,
		"remote_address": re.RemoteAddress,
		"radio_name":     re.RadioName,
		"details":        re.Details,
	} {
		if entry[key] != want {
			t.Fatalf("radio log %s changed: got %#v, want %#v", key, entry[key], want)
		}
	}

	marshaled, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}

	if bytes.Contains(marshaled, re.Raw) || bytes.Contains(marshaled, []byte(base64.StdEncoding.EncodeToString(re.Raw))) {
		t.Fatalf("raw packet bytes leaked into export")
	}
}

func TestExportSupportData_RadioLogsLimit(t *testing.T) {
	database := setupTestDB(t)

	total := 150
	for i := 1; i <= total; i++ {
		re := &dbwriter.RadioEvent{
			Timestamp:     dbwriter.EpochMillis(time.Now().UTC().Add(time.Duration(i) * time.Second)),
			Protocol:      "NGAP",
			MessageType:   fmt.Sprintf("msg-%d", i),
			Direction:     "inbound",
			LocalAddress:  "10.0.0.1:38412",
			RemoteAddress: "192.0.2.1:38412",
			Raw:           []byte("raw-bytes"),
			Details:       "test-details",
		}

		if err := database.InsertRadioEvent(context.Background(), re); err != nil {
			t.Fatalf("InsertRadioEvent failed at %d: %v", i, err)
		}
	}

	out, err := database.ExportSupportData(context.Background(), supportbundle.ExportOptions{IncludeAuditLogs: true})
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	logsAny, ok := out["radio_logs"]
	if !ok {
		t.Fatalf("radio_logs missing from export")
	}

	logsSlice, ok := logsAny.([]any)
	if !ok {
		t.Fatalf("radio_logs wrong type: %T", logsAny)
	}

	if len(logsSlice) != 100 {
		t.Fatalf("expected 100 radio logs in export, got %d", len(logsSlice))
	}

	first, ok := logsSlice[0].(map[string]any)
	if !ok {
		t.Fatalf("radio_log entry wrong type: %T", logsSlice[0])
	}

	if first["message_type"] != fmt.Sprintf("msg-%d", total) {
		t.Fatalf("unexpected first radio log message: %v", first["message_type"])
	}

	last, ok := logsSlice[99].(map[string]any)
	if !ok {
		t.Fatalf("radio_log entry wrong type: %T", logsSlice[99])
	}

	if last["message_type"] != fmt.Sprintf("msg-%d", total-99) {
		t.Fatalf("unexpected last radio log message: %v", last["message_type"])
	}
}

func readSupportBundleFiles(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()

	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	defer func() { _ = gr.Close() }()

	files := map[string][]byte{}

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}

		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", hdr.Name, err)
		}

		files[hdr.Name] = b
	}

	return files
}

func TestWriteSupportBundle_SecretsOnly(t *testing.T) {
	database := setupTestDB(t)

	operator, err := database.GetOperator(context.Background())
	if err != nil {
		t.Fatalf("GetOperator failed: %v", err)
	}

	homeKeys, err := database.ListHomeNetworkKeys(context.Background())
	if err != nil {
		t.Fatalf("ListHomeNetworkKeys failed: %v", err)
	}

	if len(homeKeys) == 0 {
		t.Fatal("expected at least one home network key")
	}

	defaultProfile, err := database.GetProfile(context.Background(), db.InitialProfileName)
	if err != nil {
		t.Fatalf("Couldn't get default profile: %v", err)
	}

	dn := &db.DataNetwork{Name: "support-net", IPv4Pool: "10.77.0.0/24", DNS: "1.1.1.1", MTU: 1400}
	if err := database.CreateDataNetwork(context.Background(), dn); err != nil {
		t.Fatalf("CreateDataNetwork failed: %v", err)
	}

	createdDNN, err := database.GetDataNetwork(context.Background(), dn.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork failed: %v", err)
	}

	imsi := "001010000000001"

	sub := &db.Subscriber{
		Imsi:           imsi,
		Msisdn:         "15551234567",
		SequenceNumber: "123456abcdef",
		PermanentKey:   strings.Repeat("p", 32),
		Opc:            strings.Repeat("o", 32),
		ProfileID:      defaultProfile.ID,
	}
	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}

	sessionID := 1

	lease := &db.IPLease{
		PoolID:    createdDNN.ID,
		PoolType:  "ipv4",
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}
	if err := database.CreateLease(context.Background(), lease, netip.MustParseAddr("10.77.0.7")); err != nil {
		t.Fatalf("CreateLease failed: %v", err)
	}

	if err := database.ReplaceFramedRoutes(context.Background(), imsi, createdDNN.ID, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}); err != nil {
		t.Fatalf("ReplaceFramedRoutes failed: %v", err)
	}

	peer := &db.BGPPeer{
		Address:     "192.0.2.1",
		RemoteAS:    65001,
		HoldTime:    90,
		Password:    "support-bgp-password-secret",
		Description: "peer for imsi-" + imsi + " admin@example.com",
	}
	if err := database.CreateBGPPeer(context.Background(), peer); err != nil {
		t.Fatalf("CreateBGPPeer failed: %v", err)
	}

	auditID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}

	al := &dbwriter.AuditLog{
		ID:        auditID.String(),
		Timestamp: dbwriter.EpochMillis(time.Now().UTC()),
		Level:     "INFO",
		Actor:     "admin@ellanetworks.com",
		Action:    "subscriber:register",
		IP:        "127.0.0.1",
		Details:   "registered subscriber " + imsi + "\n  MSISDN=" + sub.Msisdn + " admin@example.com",
	}
	if err := database.InsertAuditLog(context.Background(), al); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	radio := &dbwriter.RadioEvent{
		Timestamp:     dbwriter.EpochMillis(time.Now().UTC()),
		Protocol:      "NGAP",
		MessageType:   "InitialUEMessage",
		Direction:     "inbound",
		LocalAddress:  "10.0.0.1:38412",
		RemoteAddress: "192.0.2.1:38412",
		RadioName:     "radio-admin@example.com",
		Raw:           []byte("support-raw-packet-security-key"),
		Details:       "registration for imsi-" + imsi + "\n  MSISDN=" + sub.Msisdn,
	}
	if err := database.InsertRadioEvent(context.Background(), radio); err != nil {
		t.Fatalf("InsertRadioEvent failed: %v", err)
	}

	logMessage := "UE registration for imsi-" + imsi + " admin@example.com"
	logger.EllaLog.Info(logMessage, logger.SUPIFromIMSI(imsi))

	origAMFDumper := supportbundle.AMFDumper
	origMMEDumper := supportbundle.MMEDumper

	supportbundle.AMFDumper = func(ctx context.Context) (any, error) {
		return []map[string]any{{"supi": "imsi-" + imsi, "state": "Registered"}}, nil
	}

	supportbundle.MMEDumper = func(ctx context.Context) (any, error) {
		return []map[string]any{{"imsi": imsi, "state": "Attached"}}, nil
	}

	defer func() {
		supportbundle.AMFDumper = origAMFDumper
		supportbundle.MMEDumper = origMMEDumper
	}()

	var buf bytes.Buffer
	if err := database.WriteSupportBundle(context.Background(), &buf, supportbundle.ExportOptions{IncludeAuditLogs: true, IncludeSubscriberUsage: true}); err != nil {
		t.Fatalf("WriteSupportBundle failed: %v", err)
	}

	files := readSupportBundleFiles(t, buf.Bytes())

	var dbJSON map[string]any
	if err := json.Unmarshal(files["db.json"], &dbJSON); err != nil {
		t.Fatalf("unmarshal db.json: %v", err)
	}

	subsAny, ok := dbJSON["subscribers"].([]any)
	if !ok || len(subsAny) != 1 {
		t.Fatalf("db.json subscribers missing or wrong length: %#v", dbJSON["subscribers"])
	}

	if !reflect.DeepEqual(subsAny[0], map[string]any{"imsi": imsi, "msisdn": sub.Msisdn, "profile_id": defaultProfile.ID}) {
		t.Fatalf("db.json subscriber identities changed or non-allowlisted fields exported: %#v", subsAny[0])
	}

	for _, section := range []string{"ip_leases", "subscriber_framed_routes"} {
		entries, ok := dbJSON[section].([]any)
		if !ok || len(entries) != 1 {
			t.Fatalf("db.json %s missing or wrong length: %#v", section, dbJSON[section])
		}

		if got := entries[0].(map[string]any)["imsi"]; got != imsi {
			t.Fatalf("db.json %s imsi changed: %#v", section, got)
		}
	}

	if got := dbJSON["operator"].(map[string]any)["OperatorCode"]; got != "*" {
		t.Fatal("db.json operator code not masked")
	}

	exportedKeys, ok := dbJSON["home_network_keys"].([]any)
	if !ok || len(exportedKeys) != len(homeKeys) {
		t.Fatalf("db.json home_network_keys missing or wrong length: %#v", dbJSON["home_network_keys"])
	}

	for _, key := range exportedKeys {
		if key.(map[string]any)["PrivateKey"] != "*" {
			t.Fatal("db.json home network private key not masked")
		}
	}

	peers, ok := dbJSON["bgp_peers"].([]any)
	if !ok || len(peers) != 1 {
		t.Fatalf("db.json bgp_peers missing or wrong length: %#v", dbJSON["bgp_peers"])
	}

	if peers[0].(map[string]any)["password"] != "*" {
		t.Fatal("db.json BGP password not masked")
	}

	if peers[0].(map[string]any)["description"] != peer.Description {
		t.Fatalf("db.json BGP description changed: %#v", peers[0])
	}

	auditAny, ok := dbJSON["audit_logs"].([]any)
	if !ok || len(auditAny) != 1 {
		t.Fatalf("db.json audit_logs missing or wrong length: %#v", dbJSON["audit_logs"])
	}

	audit := auditAny[0].(map[string]any)
	for key, want := range map[string]any{
		"id":        al.ID,
		"timestamp": float64(al.Timestamp),
		"level":     al.Level,
		"actor":     al.Actor,
		"action":    al.Action,
		"ip":        al.IP,
		"details":   al.Details,
	} {
		if audit[key] != want {
			t.Fatalf("db.json audit %s changed: got %#v, want %#v", key, audit[key], want)
		}
	}

	radioAny, ok := dbJSON["radio_logs"].([]any)
	if !ok || len(radioAny) != 1 {
		t.Fatalf("db.json radio_logs missing or wrong length: %#v", dbJSON["radio_logs"])
	}

	radioEntry := radioAny[0].(map[string]any)
	if radioEntry["details"] != radio.Details || radioEntry["radio_name"] != radio.RadioName {
		t.Fatalf("db.json radio details or name changed: %#v", radioEntry)
	}

	for _, key := range []string{"Raw", "raw"} {
		if _, ok := radioEntry[key]; ok {
			t.Fatalf("db.json radio log contains %s", key)
		}
	}

	if got := dbJSON["bundle_metadata"].(map[string]any)["redaction"]; !reflect.DeepEqual(got, map[string]any{"policy": "secrets-only"}) {
		t.Fatalf("db.json unexpected redaction metadata: %#v", got)
	}

	var amfUEs []map[string]any
	if err := json.Unmarshal(files["amf_ues.json"], &amfUEs); err != nil {
		t.Fatalf("unmarshal amf_ues.json: %v", err)
	}

	if !reflect.DeepEqual(amfUEs, []map[string]any{{"supi": "imsi-" + imsi, "state": "Registered"}}) {
		t.Fatalf("amf_ues.json changed: %#v", amfUEs)
	}

	var mmeUEs []map[string]any
	if err := json.Unmarshal(files["mme_ues.json"], &mmeUEs); err != nil {
		t.Fatalf("unmarshal mme_ues.json: %v", err)
	}

	if !reflect.DeepEqual(mmeUEs, []map[string]any{{"imsi": imsi, "state": "Attached"}}) {
		t.Fatalf("mme_ues.json changed: %#v", mmeUEs)
	}

	logFound := false

	for _, line := range bytes.Split(files["logs/system.log"], []byte("\n")) {
		if len(line) == 0 {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("unmarshal system log: %v", err)
		}

		if entry["msg"] == logMessage && entry["supi"] == "imsi-"+imsi {
			logFound = true
		}
	}

	if !logFound {
		t.Fatal("logs/system.log missing original message and SUPI")
	}

	secrets := map[string]string{
		"operator code":         operator.OperatorCode,
		"subscriber key":        sub.PermanentKey,
		"subscriber OPC":        sub.Opc,
		"subscriber SQN":        sub.SequenceNumber,
		"BGP password":          peer.Password,
		"raw packet":            string(radio.Raw),
		"base64-encoded packet": base64.StdEncoding.EncodeToString(radio.Raw),
	}
	for _, key := range homeKeys {
		secrets["home network private key "+key.ID] = key.PrivateKey
	}

	for name, secret := range secrets {
		if secret == "" || secret == "*" {
			t.Fatalf("missing %s secret fixture", name)
		}
	}

	for name, content := range files {
		for secretName, secret := range secrets {
			if bytes.Contains(content, []byte(secret)) {
				t.Fatalf("%s leaked %s", name, secretName)
			}
		}
	}
}
