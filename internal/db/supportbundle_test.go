package db_test

import (
	"context"
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

		if hnKey, ok := opm["HomeNetworkPrivateKey"].(string); !ok || hnKey != "*" {
			t.Fatalf("operator HomeNetworkPrivateKey not redacted; got %#v", opm["HomeNetworkPrivateKey"])
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

	// subscribers summary
	ss, ok := out["subscribers_summary"].(map[string]any)
	if !ok {
		t.Fatalf("subscribers_summary missing or wrong type")
	}
	// totals may be numeric (float64 after JSON roundtrip)
	switch v := ss["total"].(type) {
	case int:
		// ok
		_ = v
	case float64:
		// ok
		_ = v
	default:
		t.Fatalf("subscribers_summary.total missing or wrong type: %T", ss["total"])
	}
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

	policy := &db.Policy{Name: "support-policy", BitrateUplink: "100 Mbps", BitrateDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkID: dnID}
	if err := database.CreatePolicy(context.Background(), policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// fetch policy to get ID
	policies, _, err := database.ListPoliciesPage(context.Background(), 1, 10)
	if err != nil || len(policies) == 0 {
		t.Fatalf("unable to get policy for creating subscriber: %v", err)
	}

	var policyID int

	for _, p := range policies {
		if p.Name == policy.Name {
			policyID = p.ID
			break
		}
	}

	if policyID == 0 {
		t.Fatalf("couldn't find created policy")
	}

	// create a subscriber referencing the created policy
	sub := &db.Subscriber{
		Imsi:           "001010000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   strings.Repeat("p", 32),
		Opc:            strings.Repeat("o", 32),
		PolicyID:       policyID,
	}
	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}

	out, err := database.ExportSupportData(context.Background())
	if err != nil {
		t.Fatalf("ExportSupportData failed: %v", err)
	}

	ss, ok := out["subscribers_summary"].(map[string]any)
	if !ok {
		t.Fatalf("subscribers_summary missing or wrong type")
	}
	// totals may be numeric (float64 after redaction), accept int or float64
	switch v := ss["total"].(type) {
	case int:
		if v != 1 {
			t.Fatalf("expected total subscribers 1, got %d", v)
		}
	case float64:
		if int(v) != 1 {
			t.Fatalf("expected total subscribers 1, got %v", v)
		}
	default:
		t.Fatalf("subscribers_summary.total wrong type: %T", ss["total"])
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

	if hnKey, ok := opm["HomeNetworkPrivateKey"].(string); !ok || hnKey != "*" {
		t.Fatalf("operator HomeNetworkPrivateKey not redacted; got %#v", opm["HomeNetworkPrivateKey"])
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
