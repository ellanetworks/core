package db_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

func TestExportSupportData_Default(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	out, err := database.ExportSupportData(context.Background())
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	// bundle_metadata
	bm, ok := out["bundle_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("bundle_metadata missing or wrong type")
	}

	if bm["version"] != "1.0" {
		t.Fatalf("unexpected bundle metadata version: %v", bm["version"])
	}

	if _, err := time.Parse(time.RFC3339, bm["captured_at"].(string)); err != nil {
		t.Fatalf("captured_at not a RFC3339 timestamp: %v", err)
	}

	// operator
	if opm, ok := out["operator"].(map[string]any); !ok {
		t.Fatalf("operator missing or wrong type: %T", out["operator"])
	} else {
		// Verify sensitive fields are redacted to "*"
		if opCode, ok := opm["OperatorCode"].(string); !ok || opCode != "*" {
			t.Fatalf("operator OperatorCode not redacted; got %#v", opm["OperatorCode"])
		}
	}

	// home_network_keys should be present with redacted private keys
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

	// policies (JSON-decoded) should be an array
	if policies, ok := out["policies"].([]any); !ok {
		t.Fatalf("policies missing or wrong type: %T", out["policies"])
	} else if len(policies) == 0 {
		t.Fatalf("expected at least one policy in export")
	}

	// networking
	if dns, ok := out["networking"].([]any); !ok {
		t.Fatalf("networking missing or wrong type: %T", out["networking"])
	} else if len(dns) == 0 {
		t.Fatalf("expected at least one data network in export")
	}

	// subscribers list
	subsAny, ok := out["subscribers"].([]any)
	if !ok {
		t.Fatalf("subscribers missing or wrong type: %T", out["subscribers"])
	}

	_ = subsAny
}

func TestExportSupportData_WithEntries(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	// insert an audit log
	al := &dbwriter.AuditLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "INFO",
		Actor:     "testuser",
		Action:    "test-action",
		IP:        "127.0.0.1",
		Details:   "details",
	}
	if err := database.InsertAuditLog(context.Background(), al); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	// create a data network explicitly and a policy referencing it
	dn := &db.DataNetwork{Name: "support-net", IPPool: "10.99.0.0/24", DNS: "1.1.1.1", MTU: 1400}
	if err := database.CreateDataNetwork(context.Background(), dn); err != nil {
		t.Fatalf("CreateDataNetwork failed: %v", err)
	}

	// fetch data networks to get ID
	dns, _, err := database.ListDataNetworksPage(context.Background(), 1, 10)
	if err != nil || len(dns) == 0 {
		t.Fatalf("unable to list data networks: %v", err)
	}

	var dnID int

	for _, d := range dns {
		if d.Name == dn.Name {
			dnID = d.ID
			break
		}
	}

	if dnID == 0 {
		t.Fatalf("couldn't find created data network")
	}

	policy := &db.Policy{Name: "support-policy", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkID: dnID, ProfileID: 1, SliceID: 1}
	if err := database.CreatePolicy(context.Background(), policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// create a subscriber referencing the default profile
	sub := &db.Subscriber{
		Imsi:           "001010000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   strings.Repeat("p", 32),
		Opc:            strings.Repeat("o", 32),
		ProfileID:      1,
	}
	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}

	out, err := database.ExportSupportData(context.Background())
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

	// operator's home network private key and operator code should be redacted
	opVal, exists := out["operator"]
	if !exists {
		t.Fatalf("operator missing in export")
	}

	opm, ok := opVal.(map[string]any)
	if !ok {
		t.Fatalf("operator has unexpected type: %T", opVal)
	}

	if opCode, ok := opm["OperatorCode"].(string); !ok || opCode != "*" {
		t.Fatalf("operator OperatorCode not redacted; got %#v", opm["OperatorCode"])
	}

	// home_network_keys should be present with redacted private keys
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

	// validate policies and networking content include our created entries
	polsAny, ok := out["policies"].([]any)
	if !ok {
		t.Fatalf("policies missing or wrong type: %T", out["policies"])
	}

	foundPolicy := false

	for _, pi := range polsAny {
		if pm, ok := pi.(map[string]any); ok {
			// name may be "name" or "Name" depending on marshaling
			var name string
			if v, ok := pm["name"].(string); ok {
				name = v
			} else if v, ok := pm["Name"].(string); ok {
				name = v
			}

			if name != policy.Name {
				continue
			}

			// check dataNetworkID numeric equivalence (several possible key casings)
			var idAny any
			if v, ok := pm["dataNetworkID"]; ok {
				idAny = v
			} else if v, ok := pm["DataNetworkID"]; ok {
				idAny = v
			} else if v, ok := pm["dataNetworkId"]; ok {
				idAny = v
			}

			switch idv := idAny.(type) {
			case int:
				if idv == dnID {
					foundPolicy = true
				}
			case float64:
				if int(idv) == dnID {
					foundPolicy = true
				}
			}
		}
	}

	if !foundPolicy {
		t.Fatalf("created policy not found in export")
	}

	netsAny, ok := out["networking"].([]any)
	if !ok {
		t.Fatalf("networking missing or wrong type: %T", out["networking"])
	}

	foundNet := false

	for _, ni := range netsAny {
		if nm, ok := ni.(map[string]any); ok {
			// name may be "name" or "Name"
			var name string
			if v, ok := nm["name"].(string); ok {
				name = v
			} else if v, ok := nm["Name"].(string); ok {
				name = v
			}

			if name != dn.Name {
				continue
			}

			// ipPool may be "ipPool" or "IPPool"
			var ip string
			if v, ok := nm["ipPool"].(string); ok {
				ip = v
			} else if v, ok := nm["IPPool"].(string); ok {
				ip = v
			}

			if ip == dn.IPPool {
				foundNet = true
				break
			}
		}
	}

	if !foundNet {
		t.Fatalf("created data network not found in export")
	}
}

func TestExportSupportData_IncludesRadioLogs(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	// insert a radio event
	re := &dbwriter.RadioEvent{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Protocol:      "NGAP",
		MessageType:   "InitialUEMessage",
		Direction:     "inbound",
		LocalAddress:  "10.0.0.1:38412",
		RemoteAddress: "192.0.2.1:38412",
		Raw:           []byte("raw-bytes"),
		Details:       "test-details",
	}

	if err := database.InsertRadioEvent(context.Background(), re); err != nil {
		t.Fatalf("InsertRadioEvent failed: %v", err)
	}

	out, err := database.ExportSupportData(context.Background())
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

	if len(logsSlice) == 0 {
		t.Fatalf("expected at least one radio log in export")
	}

	// inspect first entry for expected fields (accept either lower or uppercased keys)
	first, ok := logsSlice[0].(map[string]any)
	if !ok {
		t.Fatalf("radio_log entry wrong type: %T", logsSlice[0])
	}

	var proto string
	if v, ok := first["protocol"].(string); ok {
		proto = v
	} else if v, ok := first["Protocol"].(string); ok {
		proto = v
	}

	if proto != "NGAP" {
		t.Fatalf("unexpected protocol in radio log: %v", proto)
	}
}

func TestExportSupportData_RadioLogsLimit(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	// insert 150 radio events; ExportSupportData should include only the last 100
	total := 150
	for i := 1; i <= total; i++ {
		re := &dbwriter.RadioEvent{
			Timestamp:     time.Now().UTC().Add(time.Duration(i) * time.Second).Format(time.RFC3339),
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

	out, err := database.ExportSupportData(context.Background())
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

	// first should be the most recent (msg-150)
	first, ok := logsSlice[0].(map[string]any)
	if !ok {
		t.Fatalf("radio_log entry wrong type: %T", logsSlice[0])
	}

	var firstMsg string
	if v, ok := first["message_type"].(string); ok {
		firstMsg = v
	} else if v, ok := first["MessageType"].(string); ok {
		firstMsg = v
	}

	if firstMsg != fmt.Sprintf("msg-%d", total) {
		t.Fatalf("unexpected first radio log message: %v", firstMsg)
	}

	// last element (index 99) should be msg-51 (the 100th most recent)
	last, ok := logsSlice[99].(map[string]any)
	if !ok {
		t.Fatalf("radio_log entry wrong type: %T", logsSlice[99])
	}

	var lastMsg string
	if v, ok := last["message_type"].(string); ok {
		lastMsg = v
	} else if v, ok := last["MessageType"].(string); ok {
		lastMsg = v
	}

	if lastMsg != fmt.Sprintf("msg-%d", total-99) {
		t.Fatalf("unexpected last radio log message: %v", lastMsg)
	}
}
