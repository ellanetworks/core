package server_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
)

func supportBundle(url string, client *http.Client, token string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/support-bundle", nil)
	if err != nil {
		return 0, nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}

	return res.StatusCode, body, nil
}

func mapKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func TestSupportBundleEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := tempDir + "/db.sqlite3"

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Trigger support bundle successfully", func(t *testing.T) {
		statusCode, body, err := supportBundle(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't trigger support bundle: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		// parse gzip + tar and ensure db.json exists and contains bundle_metadata
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}

		defer func() { _ = gz.Close() }()

		tr := tar.NewReader(gz)
		found := false
		names := map[string]struct{}{}

		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}

			if err != nil {
				t.Fatalf("error reading tar: %v", err)
			}

			names[hdr.Name] = struct{}{}
			if hdr.Name == "db.json" {
				found = true

				data, err := io.ReadAll(tr)
				if err != nil {
					t.Fatalf("failed to read db.json from tar: %v", err)
				}

				var out map[string]any
				if err := json.Unmarshal(data, &out); err != nil {
					t.Fatalf("failed to unmarshal db.json: %v", err)
				}

				if _, ok := out["bundle_metadata"]; !ok {
					t.Fatalf("db.json missing bundle_metadata")
				}
				// if operator exists, check redaction marker
				if op, ok := out["operator"]; ok {
					switch v := op.(type) {
					case map[string]any:
						if hn, ok := v["homeNetworkPrivateKey"]; ok {
							if s, ok := hn.(string); !ok || s != "*" {
								t.Fatalf("operator homeNetworkPrivateKey not redacted: %#v", hn)
							}
						}
					}
				}
				// continue reading the rest of the tar to collect names
			}
		}

		if !found {
			t.Fatalf("db.json not found in generated support bundle")
		}

		if _, ok := names["config.yaml"]; !ok {
			t.Fatalf("support bundle missing config.yaml or system/config-info.txt; entries: %v", mapKeys(names))
		}

		if _, ok := names["system/version.txt"]; !ok {
			t.Fatalf("support bundle missing system/version.txt; entries: %v", mapKeys(names))
		}

		// Ensure there is at least one other system entry besides version (e.g., uname, df, ip_*, proc files or an error file)
		sysCount := 0

		for n := range names {
			if strings.HasPrefix(n, "system/") && n != "system/version.txt" {
				sysCount++
			}
		}

		if sysCount < 1 {
			t.Fatalf("support bundle contains no system diagnostics files; entries: %v", mapKeys(names))
		}
	})

	t.Run("2. Trigger support bundle without authorization", func(t *testing.T) {
		statusCode, _, err := supportBundle(ts.URL, client, "")
		if err != nil {
			t.Fatalf("couldn't trigger support bundle: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}
	})
}
