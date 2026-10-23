// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package supportbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestGenerateSupportBundle_IncludesConfigFromProvider(t *testing.T) {
	sampleYAML := `logging:
  system:
    level: info
    output: stdout
  api:
    cert: ella_cert.pem
    key: ella_key.pem
cluster:
  join-token: super-secret-join-token
other: value
`

	expectedYAML := `logging:
  system:
    level: info
    output: stdout
  api:
    cert: ella_cert.pem
    key: ella_key.pem
cluster:
  join-token: "*"
other: value
`

	old := ConfigProvider
	ConfigProvider = func(ctx context.Context) ([]byte, error) {
		return []byte(sampleYAML), nil
	}

	defer func() { ConfigProvider = old }()

	data := map[string]any{"bundle_metadata": map[string]any{"version": "1.0"}}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	files := readBundleFiles(t, buf.Bytes())
	if string(files["config.yaml"]) != expectedYAML {
		t.Fatalf("expected content to be: %s, got: %s", expectedYAML, files["config.yaml"])
	}

	var statuses map[string]SectionStatus
	if err := json.Unmarshal(files["collection_status.json"], &statuses); err != nil {
		t.Fatal(err)
	}

	if want := (SectionStatus{Status: "complete", Count: 1}); statuses["config.yaml"] != want {
		t.Fatalf("wrong config status: got %#v, want %#v", statuses["config.yaml"], want)
	}
}

type failingBundleWriter struct{}

func (failingBundleWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestBundleOutputFailurePropagates(t *testing.T) {
	if err := GenerateSupportBundleFromData(context.Background(), SanitizedData{}, failingBundleWriter{}); err == nil {
		t.Fatal("archive output failure was ignored")
	}
}

func TestGenerateSupportBundle_ConfigFailuresSafe(t *testing.T) {
	old := ConfigProvider

	t.Cleanup(func() { ConfigProvider = old })

	for _, tc := range []struct {
		name      string
		config    string
		err       error
		reason    string
		errorText string
	}{
		{"invalid YAML", "cluster:\n  join-token: \"config-secret-marker\n", nil, "sanitization failed", "cannot parse YAML configuration safely"},
		{"unsafe token", "cluster:\n  join-token: [config-secret-marker]\n", nil, "sanitization failed", "cannot safely mask YAML join token"},
		{"collection failed", "cluster:\n  join-token: config-secret-marker\n", errors.New("config-secret-marker"), "collection failed", "Configuration collection failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ConfigProvider = func(context.Context) ([]byte, error) { return []byte(tc.config), tc.err }

			var buf bytes.Buffer
			if err := GenerateSupportBundleFromData(context.Background(), SanitizedData{}, &buf); err != nil {
				t.Fatal(err)
			}

			files := readBundleFiles(t, buf.Bytes())
			for name, body := range files {
				if bytes.Contains(body, []byte("config-secret-marker")) {
					t.Fatalf("%s leaked raw configuration: %s", name, body)
				}
			}

			if _, ok := files["config.yaml"]; ok {
				t.Fatal("unsafe configuration included as config.yaml")
			}

			if string(files["system/error-config.txt"]) != tc.errorText {
				t.Fatalf("unexpected config error: %q", files["system/error-config.txt"])
			}

			var statuses map[string]SectionStatus
			if err := json.Unmarshal(files["collection_status.json"], &statuses); err != nil {
				t.Fatal(err)
			}

			if want := (SectionStatus{Status: "failed", Reason: tc.reason}); statuses["config.yaml"] != want {
				t.Fatalf("wrong config status: got %#v, want %#v", statuses["config.yaml"], want)
			}
		})
	}
}

func TestGenerateSupportBundle_LiveProviderOutcomes(t *testing.T) {
	oldAMF, oldMME := AMFDumper, MMEDumper

	t.Cleanup(func() { AMFDumper, MMEDumper = oldAMF, oldMME })

	type liveUE struct {
		SUPI string `json:"supi"`
	}

	type liveUEs []liveUE

	for _, tc := range []struct {
		name        string
		unavailable bool
		data        any
		err         error
		want        SectionStatus
	}{
		{name: "provider unavailable", unavailable: true, want: SectionStatus{Status: "omitted", Reason: "provider unavailable"}},
		{name: "nil state", want: SectionStatus{Status: "omitted", Reason: "state unavailable"}},
		{name: "nil slice", data: liveUEs(nil), want: SectionStatus{Status: "complete", Count: 0}},
		{name: "empty slice", data: liveUEs{}, want: SectionStatus{Status: "complete", Count: 0}},
		{name: "multiple UEs", data: liveUEs{{SUPI: "imsi-001010000000001"}, {SUPI: "imsi-001010000000002"}}, want: SectionStatus{Status: "complete", Count: 2}},
		{name: "UE array", data: [2]liveUE{{SUPI: "imsi-001010000000001"}, {SUPI: "imsi-001010000000002"}}, want: SectionStatus{Status: "complete", Count: 2}},
		{name: "single object", data: liveUE{SUPI: "imsi-001010000000001"}, want: SectionStatus{Status: "complete", Count: 1}},
		{name: "collection failed", data: liveUEs{{SUPI: "imsi-001010000000001"}}, err: errors.New("provider failed for imsi-001010000000001"), want: SectionStatus{Status: "failed", Reason: "collection failed"}},
		{name: "unsupported value", data: []any{make(chan int)}, want: SectionStatus{Status: "failed", Reason: "encoding failed"}},
		{name: "invalid JSON marshaler", data: json.RawMessage(`{"invalid`), want: SectionStatus{Status: "failed", Reason: "encoding failed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			AMFDumper = func(context.Context) (any, error) { return tc.data, tc.err }
			if tc.unavailable {
				AMFDumper = nil
			}

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

			for _, provider := range []string{"amf", "mme"} {
				name := provider + "_ues.json"
				if statuses[name] != tc.want {
					t.Fatalf("wrong %s status: got %#v, want %#v", name, statuses[name], tc.want)
				}

				if tc.want.Status == "complete" {
					original, err := json.MarshalIndent(tc.data, "", "  ")
					if err != nil {
						t.Fatal(err)
					}

					if !bytes.Equal(files[name], original) {
						t.Fatalf("%s changed live data: got %s, want %s", name, files[name], original)
					}
				} else if _, ok := files[name]; ok {
					t.Fatalf("unexpected %s for %s", name, tc.want.Status)
				}

				errorName := provider + "_ues_error.txt"
				if tc.err != nil {
					if want := strings.ToUpper(provider) + " collection failed"; string(files[errorName]) != want {
						t.Fatalf("unexpected %s: got %q, want %q", errorName, files[errorName], want)
					}
				} else if _, ok := files[errorName]; ok {
					t.Fatalf("unexpected %s", errorName)
				}
			}
		})
	}
}
