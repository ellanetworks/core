// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package supportbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/logger"
)

func TestCollectionOutcomes(t *testing.T) {
	oldBPF, oldAMF, oldMME := BpfDumper, AMFDumper, MMEDumper

	t.Cleanup(func() { BpfDumper, AMFDumper, MMEDumper = oldBPF, oldAMF, oldMME })

	for _, tc := range []struct {
		name       string
		bpf        SectionStatus
		bpfError   error
		live       any
		liveStatus string
		count      int
	}{
		{"unavailable", SectionStatus{Status: "omitted", Reason: "BPF objects unavailable"}, nil, nil, "omitted", 0},
		{"partial", SectionStatus{Status: "partial", Count: 3, Reason: "maps failed or truncated"}, nil, []any{map[string]string{"event": "{invalid"}}, "complete", 1},
		{"failed", SectionStatus{}, errors.New("failure"), []any{map[string]string{"supi": "imsi-001010000000001"}}, "complete", 1},
		{"failed after capture", SectionStatus{Count: 2}, errors.New("failure"), []any{}, "complete", 0},
		{"deep content", SectionStatus{Status: "complete"}, nil, nestedLiveState(70), "complete", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			BpfDumper = func(context.Context, *tar.Writer) (SectionStatus, error) { return tc.bpf, tc.bpfError }
			AMFDumper = func(context.Context) (any, error) { return tc.live, nil }
			MMEDumper = AMFDumper

			var buf bytes.Buffer
			if err := GenerateSupportBundleFromData(context.Background(), SanitizedData{}, &buf); err != nil {
				t.Fatal(err)
			}

			files := readBundleFiles(t, buf.Bytes())

			var statuses map[string]SectionStatus
			if err := json.Unmarshal(files["collection_status.json"], &statuses); err != nil {
				t.Fatal(err)
			}

			if bytes.Contains(files["collection_status.json"], []byte("omitted_values")) {
				t.Fatal("collection manifest contains obsolete omission counts")
			}

			for _, name := range []string{"amf_ues.json", "mme_ues.json"} {
				want := SectionStatus{Status: tc.liveStatus, Count: tc.count}
				if tc.live == nil {
					want.Reason = "state unavailable"
				}

				if statuses[name] != want {
					t.Fatalf("wrong %s status: got %#v, want %#v", name, statuses[name], want)
				}

				if tc.live == nil {
					if _, ok := files[name]; ok {
						t.Fatalf("unavailable state included as %s", name)
					}

					continue
				}

				original, err := json.MarshalIndent(tc.live, "", "  ")
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(files[name], original) {
					t.Fatalf("%s changed live data: got %s, want %s", name, files[name], original)
				}
			}

			want := tc.bpf
			if tc.bpfError != nil {
				want.Status, want.Reason = "failed", "collection failed"
				if want.Count > 0 {
					want.Status = "partial"
				}

				if string(files["bpf/error.txt"]) != "BPF collection failed" {
					t.Fatalf("unexpected BPF error: %q", files["bpf/error.txt"])
				}
			}

			if statuses["bpf"] != want {
				t.Fatalf("wrong BPF status: got %#v, want %#v", statuses["bpf"], want)
			}
		})
	}
}

func nestedLiveState(depth int) any {
	var value any = "a1b2c3d4"
	for range depth {
		value = []any{value}
	}

	return value
}

func TestSectionCountPreservesIncompleteCollection(t *testing.T) {
	for _, tc := range []struct {
		initial string
		count   int
		want    string
	}{
		{"", 0, "complete"},
		{"failed", 0, "failed"},
		{"failed", 4, "partial"},
		{"partial", 0, "partial"},
	} {
		status := (SectionStatus{Status: tc.initial, Reason: "collection failed"}).WithCount(tc.count)
		if status.Status != tc.want || status.Count != tc.count {
			t.Fatalf("wrong count transition: %#v", status)
		}
	}
}

func TestGenerateSupportBundleFromData(t *testing.T) {
	captured := time.Now().UTC().Format(time.RFC3339)
	data := map[string]any{
		"bundle_metadata": map[string]any{"version": "1.0", "captured_at": captured},
		"operator":        map[string]any{"operatorCode": "*", "homeNetworkPrivateKey": "*"},
		"policies":        []any{map[string]any{"name": "p1"}},
		"networking":      []any{map[string]any{"name": "net1"}},
		"subscribers":     []any{map[string]any{"imsi": "001010000000001", "ip_address": "10.0.0.1"}},
		"audit_logs":      []any{map[string]any{"actor": "admin@example.com", "details": `{"imsi":"001010000000001","event":"{invalid"}`}},
	}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	files := readBundleFiles(t, buf.Bytes())

	original, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(files["db.json"], original) {
		t.Fatalf("db.json changed input data: got %s, want %s", files["db.json"], original)
	}
}

func TestGenerateSupportBundleFromData_IncludesRecentLogs(t *testing.T) {
	logger.EllaLog.Info("recent logs bundle marker")

	data := map[string]any{"bundle_metadata": map[string]any{"version": "1.0"}}

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
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}

		if hdr.Name != "logs/system.log" {
			continue
		}

		found = true

		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read logs/system.log: %v", err)
		}

		if !strings.Contains(string(b), "recent logs bundle marker") {
			t.Fatalf("logs/system.log does not contain the marker line: %q", b)
		}
	}

	if !found {
		t.Fatalf("logs/system.log not found in archive")
	}
}

func TestGenerateSupportBundleFromData_LogsUnchanged(t *testing.T) {
	logger.EllaLog.Info("registration", logger.SUPI("imsi-001010000000001"))
	logger.EllaLog.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")

	original := logger.RecentLogs()

	data := map[string]any{"bundle_metadata": map[string]any{"version": "1.0"}}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	files := readBundleFiles(t, buf.Bytes())
	if !bytes.Equal(files["logs/system.log"], original) {
		t.Fatalf("logs/system.log changed recent logs: got %s, want %s", files["logs/system.log"], original)
	}

	for _, expected := range []string{`"supi":"imsi-001010000000001"`, `"msg":"[NAS] UE Context released, abort retransmission of Registration Accept"`} {
		if !bytes.Contains(files["logs/system.log"], []byte(expected)) {
			t.Fatalf("logs/system.log missing original content %q: %s", expected, files["logs/system.log"])
		}
	}
}

func TestGenerateSupportBundleFromData_LiveUEsUnchanged(t *testing.T) {
	oldAMF, oldMME := AMFDumper, MMEDumper

	t.Cleanup(func() { AMFDumper, MMEDumper = oldAMF, oldMME })

	live := []map[string]any{
		{
			"identity": map[string]any{
				"supi":     "imsi-001010000000001",
				"imsi":     "001010000000001",
				"msisdn":   "15551234567",
				"pei":      "imei-350000000000001",
				"guti":     "285-01-0000-0000",
				"old_guti": "285-01-0000-0001",
				"tmsi":     "a1b2c3d4",
				"old_tmsi": "b2c3d4e5",
				"suci":     "suci-001-01-0000000000-0-0-0000000001",
			},
			"details":              ` { "supi": "imsi-001010000000001", "actor": "admin@example.com", "counter": 18446744073709551615 } `,
			"event":                "[NAS] UE Context released, abort retransmission of Registration Accept",
			"message":              "subscriber 001010000000001, imei-350000000000001, admin@example.com",
			"invalid_json":         "{invalid",
			"imsi-001010000000001": "identity in a map key",
			"counter":              uint64(18446744073709551615),
		},
	}

	AMFDumper = func(context.Context) (any, error) { return live, nil }
	MMEDumper = AMFDumper

	data := map[string]any{"bundle_metadata": map[string]any{"version": "1.0"}}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	files := readBundleFiles(t, buf.Bytes())

	original, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"amf_ues.json", "mme_ues.json"} {
		if !bytes.Equal(files[name], original) {
			t.Fatalf("%s changed live data: got %s, want %s", name, files[name], original)
		}
	}
}

func TestGenerateSupportBundleFromData_AMFUesSeparateFile(t *testing.T) {
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

			supi, ok := entry["supi"].(string)
			if !ok {
				t.Fatalf("unexpected supi: %#v", entry["supi"])
			}

			if supi != "imsi-001010000000099" {
				t.Fatalf("unexpected supi: %s", supi)
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

func readBundleFiles(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()

	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = gr.Close() }()

	files := map[string][]byte{}

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files
		}

		if err != nil {
			t.Fatal(err)
		}

		files[hdr.Name], err = io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
	}
}
