package supportbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestGenerateSupportBundleFromData(t *testing.T) {
	captured := time.Now().UTC().Format(time.RFC3339)
	data := map[string]any{
		"bundle_metadata": map[string]any{"version": "1.0", "captured_at": captured},
		"operator":        map[string]any{"operatorCode": "*", "homeNetworkPrivateKey": "*"},
		"policies":        []any{map[string]any{"name": "p1"}},
		"networking":      []any{map[string]any{"name": "net1"}},
		"subscribers":     []any{map[string]any{"imsi": "001010000000001", "ip_address": "10.0.0.1"}},
	}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	// open gzip
	gr, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}

		if hdr.Name != "db.json" {
			continue
		}

		found = true

		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read db.json: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal db.json: %v", err)
		}

		// basic key presence
		if _, ok := got["bundle_metadata"]; !ok {
			t.Fatalf("db.json missing bundle_metadata")
		}

		if _, ok := got["operator"]; !ok {
			t.Fatalf("db.json missing operator")
		}

		if _, ok := got["policies"]; !ok {
			t.Fatalf("db.json missing policies")
		}

		if _, ok := got["networking"]; !ok {
			t.Fatalf("db.json missing networking")
		}

		if subsAny, ok := got["subscribers"].([]any); !ok {
			t.Fatalf("db.json subscribers missing or wrong type")
		} else if len(subsAny) != 1 {
			t.Fatalf("unexpected subscribers length: %d", len(subsAny))
		} else {
			if first, ok := subsAny[0].(map[string]any); !ok {
				t.Fatalf("subscriber entry wrong type: %T", subsAny[0])
			} else if imsi, ok := first["imsi"].(string); !ok || imsi != "001010000000001" {
				t.Fatalf("unexpected subscriber imsi: %#v", first["imsi"])
			}
		}

		// check operator redaction marker preserved
		if opm, ok := got["operator"].(map[string]any); ok {
			if oc, ok := opm["operatorCode"].(string); !ok || oc != "*" {
				t.Fatalf("operatorCode not preserved as redaction marker: %#v", opm["operatorCode"])
			}
		} else {
			t.Fatalf("operator has unexpected type: %T", got["operator"])
		}
	}

	if !found {
		t.Fatalf("db.json not found in archive")
	}
}

func TestGenerateSupportBundleFromData_AMFUesSeparateFile(t *testing.T) {
	// Set AMFDumper to return test data.
	origDumper := AMFDumper

	defer func() { AMFDumper = origDumper }()

	AMFDumper = func(ctx context.Context) (any, error) {
		return []map[string]any{
			{"supi": "imsi-001010000000099", "state": "Registered"},
		}, nil
	}

	captured := time.Now().UTC().Format(time.RFC3339)
	data := map[string]any{
		"bundle_metadata": map[string]any{"version": "1.0", "captured_at": captured},
		"subscribers":     []any{},
	}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	gr, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	foundDBJSON := false
	foundAMFUes := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}

		switch hdr.Name {
		case "db.json":
			foundDBJSON = true

			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read db.json: %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal db.json: %v", err)
			}

			// db.json must NOT contain amf_ues
			if _, ok := got["amf_ues"]; ok {
				t.Fatalf("db.json should not contain amf_ues key")
			}

		case "amf_ues.json":
			foundAMFUes = true

			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read amf_ues.json: %v", err)
			}

			var got []any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal amf_ues.json: %v", err)
			}

			if len(got) != 1 {
				t.Fatalf("expected 1 AMF UE entry, got %d", len(got))
			}

			entry, ok := got[0].(map[string]any)
			if !ok {
				t.Fatalf("amf_ues entry wrong type: %T", got[0])
			}

			if supi, ok := entry["supi"].(string); !ok || supi != "imsi-001010000000099" {
				t.Fatalf("unexpected supi: %#v", entry["supi"])
			}
		}
	}

	if !foundDBJSON {
		t.Fatalf("db.json not found in archive")
	}

	if !foundAMFUes {
		t.Fatalf("amf_ues.json not found in archive")
	}
}
