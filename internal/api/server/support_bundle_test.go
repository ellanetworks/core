// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/supportbundle"
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

func readBundleEntries(t *testing.T, body []byte) map[string][]byte {
	t.Helper()

	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}

	defer func() { _ = gz.Close() }()

	entries := map[string][]byte{}

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("error reading tar: %v", err)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("failed to read %s from tar: %v", hdr.Name, err)
		}

		entries[hdr.Name] = data
	}

	return entries
}

func TestSupportBundleEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := tempDir + "/db.sqlite3"

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Trigger support bundle successfully", func(t *testing.T) {
		statusCode, body, err := supportBundle(env.Server.URL, client, token)
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
		statusCode, _, err := supportBundle(env.Server.URL, client, "")
		if err != nil {
			t.Fatalf("couldn't trigger support bundle: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}
	})

	t.Run("3. Sequential bundles preserve identities and restrict audit logs", func(t *testing.T) {
		managerEmail := "manager@ellanetworks.com"
		managerClient := newTestClient(env.Server)

		managerToken, err := createUserAndLogin(env.Server.URL, token, managerEmail, RoleNetworkManager, managerClient)
		if err != nil {
			t.Fatalf("couldn't create network manager: %s", err)
		}

		statusCode, body, err := supportBundle(env.Server.URL, managerClient, managerToken)
		if err != nil {
			t.Fatalf("couldn't trigger support bundle as network manager: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		managerEntries := readBundleEntries(t, body)

		var managerDB map[string]any
		if err := json.Unmarshal(managerEntries["db.json"], &managerDB); err != nil {
			t.Fatalf("failed to unmarshal db.json: %v", err)
		}

		if _, ok := managerDB["audit_logs"]; ok {
			t.Fatalf("audit_logs must not be included in a bundle for a role without the audit log listing permission")
		}

		sysLog := string(managerEntries["logs/system.log"])
		if !strings.Contains(sysLog, `"user":"`+FirstUserEmail+`"`) {
			t.Fatalf("previous bundle requester missing from logs/system.log:\n%s", sysLog)
		}

		statusCode, body, err = supportBundle(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't trigger support bundle as admin: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		adminEntries := readBundleEntries(t, body)

		var adminDB map[string]any
		if err := json.Unmarshal(adminEntries["db.json"], &adminDB); err != nil {
			t.Fatalf("failed to unmarshal db.json: %v", err)
		}

		auditAny, ok := adminDB["audit_logs"].([]any)
		if !ok || len(auditAny) == 0 {
			t.Fatalf("audit_logs missing for admin bundle: %T", adminDB["audit_logs"])
		}

		foundBundleAction := false

		for _, a := range auditAny {
			entry, ok := a.(map[string]any)
			if !ok {
				t.Fatalf("audit entry wrong type: %T", a)
			}

			if entry["action"] != "support_bundle:generate" {
				continue
			}

			foundBundleAction = true

			actor, _ := entry["actor"].(string)
			if actor != FirstUserEmail && actor != managerEmail {
				t.Fatalf("support bundle audit actor changed: %#v", entry["actor"])
			}
		}

		if !foundBundleAction {
			t.Fatalf("audit_logs missing support_bundle:generate entry")
		}

		for _, email := range []string{FirstUserEmail, managerEmail} {
			if !bytes.Contains(adminEntries["db.json"], []byte(email)) {
				t.Fatalf("audit logs did not retain email %q", email)
			}
		}
	})
}

func TestSupportBundleSubscriberUsagePermission(t *testing.T) {
	env, client, adminToken := newAuthedTestEnv(t)
	ctx := context.Background()

	profile, err := env.DB.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatal(err)
	}

	imsi := "001010100007487"
	if err := env.DB.CreateSubscriber(ctx, &db.Subscriber{
		Imsi: imsi, ProfileID: profile.ID, PermanentKey: Key, Opc: Opc, SequenceNumber: SequenceNumber,
	}); err != nil {
		t.Fatal(err)
	}

	day := time.Now().UTC().AddDate(0, 0, -1)

	usage := db.DailyUsage{EpochDay: db.DaysSinceEpoch(day), IMSI: imsi, BytesUplink: 1500, BytesDownlink: 2500}
	if err := env.DB.IncrementDailyUsage(ctx, usage); err != nil {
		t.Fatal(err)
	}

	managerToken, err := createUserAndLogin(env.Server.URL, adminToken, "usage-manager@example.com", RoleNetworkManager, newTestClient(env.Server))
	if err != nil {
		t.Fatal(err)
	}

	for _, role := range []struct {
		name    string
		token   string
		allowed bool
	}{
		{name: "network_manager", token: managerToken},
		{name: "admin", token: adminToken, allowed: true},
	} {
		t.Run(role.name, func(t *testing.T) {
			status, apiToken, err := createAPIToken(env.Server.URL, client, role.token, &CreateAPITokenParams{Name: "support-usage"})
			if err != nil {
				t.Fatal(err)
			}

			if status != http.StatusCreated || apiToken.Result.Token == "" {
				t.Fatalf("create API token: status=%d response=%+v", status, apiToken)
			}

			for _, auth := range []struct {
				name  string
				token string
			}{
				{name: "jwt", token: role.token},
				{name: "api_token", token: apiToken.Result.Token},
			} {
				t.Run(auth.name, func(t *testing.T) {
					client := newTestClient(env.Server)

					wantStatus := http.StatusForbidden
					if role.allowed {
						wantStatus = http.StatusOK
					}

					status, daily, err := getUsagePerDay(env.Server.URL, client, auth.token, "", "", "")
					if err != nil {
						t.Fatal(err)
					}

					if status != wantStatus {
						t.Fatalf("direct daily usage: got status %d, want %d", status, wantStatus)
					}

					status, subscribers, err := getUsagePerSubscriber(env.Server.URL, client, auth.token, "", "", "")
					if err != nil {
						t.Fatal(err)
					}

					if status != wantStatus {
						t.Fatalf("direct per-subscriber usage: got status %d, want %d", status, wantStatus)
					}

					t.Logf("both direct usage endpoints returned %d", wantStatus)

					if role.allowed {
						got, ok := usageForDate(daily.Result, day.Format("2006-01-02"))

						wantDay := DailySubscriberUsage{Date: day.Format("2006-01-02"), UplinkBytes: 1500, DownlinkBytes: 2500, TotalBytes: 4000}
						if !ok || got != wantDay {
							t.Fatalf("direct daily usage changed: %+v", daily.Result)
						}

						wantSubscribers := []PerSubscriberUsage{{IMSI: imsi, UplinkBytes: 1500, DownlinkBytes: 2500, TotalBytes: 4000}}
						if !reflect.DeepEqual(subscribers.Result, wantSubscribers) {
							t.Fatalf("direct per-subscriber usage changed: %+v", subscribers.Result)
						}
					}

					status, body, err := supportBundle(env.Server.URL, client, auth.token)
					if err != nil {
						t.Fatal(err)
					}

					if status != http.StatusOK {
						t.Fatalf("support bundle: got status %d, want 200", status)
					}

					var out map[string]json.RawMessage
					if err := json.Unmarshal(readBundleEntries(t, body)["db.json"], &out); err != nil {
						t.Fatal(err)
					}

					var metadata struct {
						Sections map[string]supportbundle.SectionStatus `json:"sections"`
					}
					if err := json.Unmarshal(out["bundle_metadata"], &metadata); err != nil {
						t.Fatal(err)
					}

					for section, want := range map[string][]map[string]any{
						"usage_per_day":        {{"day": float64(usage.EpochDay), "bytes_uplink": float64(1500), "bytes_downlink": float64(2500)}},
						"usage_per_subscriber": {{"imsi": imsi, "bytes_uplink": float64(1500), "bytes_downlink": float64(2500)}},
					} {
						if !role.allowed {
							if data, ok := out[section]; ok {
								t.Errorf("%s exported without subscriber usage permission: %s", section, data)
							}

							if got := metadata.Sections[section]; got != (supportbundle.SectionStatus{Status: "omitted", Reason: "permission restricted"}) {
								t.Errorf("unexpected %s status: %+v", section, got)
							}

							continue
						}

						var got []map[string]any
						if err := json.Unmarshal(out[section], &got); err != nil {
							t.Fatalf("decode %s: %v", section, err)
						}

						if !reflect.DeepEqual(got, want) {
							t.Errorf("bundle %s changed: got %+v, want %+v", section, got, want)
						}

						if got := metadata.Sections[section]; got != (supportbundle.SectionStatus{Status: "complete", Count: 1}) {
							t.Errorf("unexpected %s status: %+v", section, got)
						}
					}
				})
			}
		})
	}
}
