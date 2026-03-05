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
		"bundle_metadata":     map[string]any{"version": "1.0", "captured_at": captured},
		"operator":            map[string]any{"operatorCode": "*", "homeNetworkPrivateKey": "*"},
		"policies":            []any{map[string]any{"name": "p1"}},
		"networking":          []any{map[string]any{"name": "net1"}},
		"subscribers_summary": map[string]any{"total": 1, "with_ip": 1},
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

		if ss, ok := got["subscribers_summary"].(map[string]any); !ok {
			t.Fatalf("db.json subscribers_summary missing or wrong type")
		} else {
			if total, ok := ss["total"].(float64); !ok || int(total) != 1 {
				t.Fatalf("unexpected subscribers_summary.total: %#v", ss["total"])
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
