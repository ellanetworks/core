// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

var updateSchemaGolden = flag.Bool("update", false, "rewrite testdata/schema.golden.sql")

const (
	schemaGoldenPath  = "testdata/schema.golden.sql"
	releaseFixtureDir = "testdata/releases"
	releaseFixtureDB  = "core.db"
)

type releaseFixture struct {
	version     string
	schema      int
	unpopulated map[string]string
	assert      func(t *testing.T, conn *sql.DB)
}

var releaseFixtures = []releaseFixture{
	{
		version: "v1.21.0",
		schema:  25,
		unpopulated: map[string]string{
			"cluster_join_hmac":           "standalone fixture",
			"cluster_join_tokens":         "standalone fixture",
			"cluster_members":             "standalone fixture",
			"cluster_node_certs":          "standalone fixture",
			"deferred_location_requests":  "needs a location client",
			"ims_registration_sets":       "needs an IMS",
			"ims_registrations":           "needs an IMS",
			"ip_sm_gw_registrations":      "needs an IP-SM-GW",
			"location_reports":            "needs a location client",
			"sms_message_waiting":         "needs an SMSC",
			"sms_message_waiting_centres": "needs an SMSC",
			"subscriber_framed_routes":    "not seeded",
		},
		assert: assertReleaseFixtureV1_21_0,
	},
}

func assertReleaseFixtureV1_21_0(t *testing.T, conn *sql.DB) {
	t.Helper()

	assertRows(t, conn, "slice data networks",
		`SELECT s.name, d.name, sdn.nat, sdn.localSwitch, sdn.networkInstance
		FROM slice_data_networks sdn
		JOIN network_slices s ON s.id = sdn.sliceID
		JOIN data_networks d ON d.id = sdn.dataNetworkID
		ORDER BY s.name, d.name`,
		[]string{
			"default|ims|false|true|default",
			"default|internet|false|true|default",
			"iot|internet|false|true|default",
		})

	assertRows(t, conn, "SMSCs",
		`SELECT s.name, p.diameterIdentity, p.address, p.port
		FROM sms_smscs s JOIN diameter_peers p ON p.smscId = s.id
		ORDER BY s.name`,
		[]string{
			"192.0.2.11||192.0.2.11|3870",
			"smsc.example.org|smsc.example.org|192.0.2.10|3868",
		})

	assertRows(t, conn, "SMSC service centres",
		`SELECT s.name, c.serviceCentre
		FROM sms_smsc_service_centres c JOIN sms_smscs s ON s.id = c.smscId
		ORDER BY c.serviceCentre`,
		[]string{
			"smsc.example.org|15550000000",
			"192.0.2.11|15550000001",
			"smsc.example.org|15550000009",
		})

	assertRows(t, conn, "P-CSCF addresses",
		`SELECT address FROM pcscf_addresses ORDER BY priority`,
		[]string{"10.46.0.250"})

	assertRows(t, conn, "IMS application servers",
		`SELECT name, serverName FROM ims_application_servers`,
		[]string{"mcptt|sip:mcptt.ims.example.org"})

	assertRows(t, conn, "static IPs",
		`SELECT imsi, type FROM ip_leases WHERE type = 'static'`,
		[]string{"001017271246547|static"})
}

func TestReleaseFixturesMigrateToCurrentSchema(t *testing.T) {
	golden := readSchemaGolden(t)

	for _, fx := range releaseFixtures {
		t.Run(fx.version, func(t *testing.T) {
			dir := extractReleaseFixture(t, fx.version)
			dbPath := filepath.Join(dir, releaseFixtureDB)

			before := openRawDB(t, dbPath)

			if got := schemaVersionOf(t, before); got != fx.schema {
				t.Fatalf("fixture schema = %d, want %d", got, fx.schema)
			}

			countsBefore := tableRowCounts(t, before)

			assertFixtureCoverage(t, countsBefore, fx.unpopulated)

			if err := before.Close(); err != nil {
				t.Fatalf("close fixture: %v", err)
			}

			database, err := NewDatabase(context.Background(), dbPath, ellaraft.FastTestConfig())
			if err != nil {
				t.Fatalf("open %s fixture with this build: %v", fx.version, err)
			}

			t.Cleanup(func() { _ = database.Close() })

			if err := database.WaitUntilReady(t.Context()); err != nil {
				t.Fatalf("database never became ready: %v", err)
			}

			waitForSchema(t, database, SchemaVersion())

			conn := database.conn().PlainDB()

			assertIntegrity(t, conn)

			if got := dumpSchema(t, conn); got != golden {
				t.Fatalf("schema migrated from %s differs from %s; run go test ./internal/db -run TestSchemaGolden -update if the change is intended", fx.version, schemaGoldenPath)
			}

			countsAfter := tableRowCounts(t, conn)

			for table, n := range countsBefore {
				if n == 0 {
					continue
				}

				after, kept := countsAfter[table]
				if !kept {
					continue
				}

				if after < n {
					t.Errorf("table %s lost rows during the upgrade: %d -> %d", table, n, after)
				}
			}

			fx.assert(t, conn)
		})
	}
}

func TestSchemaGolden(t *testing.T) {
	database := newStandaloneDB(t)

	got := dumpSchema(t, database.conn().PlainDB())

	if *updateSchemaGolden {
		if err := os.WriteFile(schemaGoldenPath, []byte(got), 0o600); err != nil {
			t.Fatalf("write %s: %v", schemaGoldenPath, err)
		}

		return
	}

	if got != readSchemaGolden(t) {
		t.Fatalf("schema differs from %s; a shipped migration must never change. Run go test ./internal/db -run TestSchemaGolden -update if you added a migration", schemaGoldenPath)
	}
}

func TestOpenRefusesDatabaseNewerThanBinary(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite3")

	database, err := NewDatabase(context.Background(), dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	waitForSchema(t, database, SchemaVersion())

	if err := database.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	raw := openRawDB(t, dbPath)

	if _, err := raw.ExecContext(t.Context(), "UPDATE schema_version SET version = ? WHERE id = 1", SchemaVersion()+1); err != nil {
		t.Fatalf("bump schema_version: %v", err)
	}

	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	reopened, err := NewDatabase(context.Background(), dbPath, ellaraft.FastTestConfig())
	if err == nil {
		_ = reopened.Close()

		t.Fatal("a database newer than this binary was opened")
	}

	if !errors.Is(err, ErrSchemaNewerThanBinary) {
		t.Fatalf("error = %v, want %v", err, ErrSchemaNewerThanBinary)
	}

	raw = openRawDB(t, dbPath)

	if got := schemaVersionOf(t, raw); got != SchemaVersion()+1 {
		t.Fatalf("refused database was modified: schema_version = %d", got)
	}

	_ = raw.Close()
}

func assertFixtureCoverage(t *testing.T, counts map[string]int, unpopulated map[string]string) {
	t.Helper()

	for table, n := range counts {
		_, allowed := unpopulated[table]

		switch {
		case n == 0 && !allowed:
			t.Errorf("fixture table %s is empty; seed it when generating the fixture or list it as unpopulated", table)
		case n > 0 && allowed:
			t.Errorf("fixture table %s has rows but is listed as unpopulated", table)
		}
	}

	for table := range unpopulated {
		if _, ok := counts[table]; !ok {
			t.Errorf("unpopulated table %s does not exist in the fixture", table)
		}
	}
}

func assertRows(t *testing.T, conn *sql.DB, what, query string, want []string) {
	t.Helper()

	rows, err := conn.QueryContext(t.Context(), query)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}

	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}

	var got []string

	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))

		for i := range vals {
			ptrs[i] = &vals[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}

		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = v.String
		}

		got = append(got, strings.Join(parts, "|"))
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("%s: %v", what, err)
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s:\n got: %q\nwant: %q", what, got, want)
	}
}

func assertIntegrity(t *testing.T, conn *sql.DB) {
	t.Helper()

	var result string
	if err := conn.QueryRowContext(t.Context(), "PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}

	if result != "ok" {
		t.Fatalf("integrity_check: %s", result)
	}

	rows, err := conn.QueryContext(t.Context(), "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}

	defer func() { _ = rows.Close() }()

	if rows.Next() {
		var table string

		var rowid sql.NullInt64

		var parent string

		var fkid int

		_ = rows.Scan(&table, &rowid, &parent, &fkid)
		t.Fatalf("foreign key violation: %s row %d references missing %s", table, rowid.Int64, parent)
	}
}

func waitForSchema(t *testing.T, database *Database, want int) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for {
		got, err := database.CurrentSchemaVersion(t.Context())
		if err == nil && got == want {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("schema version = %d (err %v), want %d", got, err, want)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

var (
	schemaWhitespace  = regexp.MustCompile(`\s+`)
	schemaPunctuation = regexp.MustCompile(`\s*([,()])\s*`)
)

func dumpSchema(t *testing.T, conn *sql.DB) string {
	t.Helper()

	rows, err := conn.QueryContext(t.Context(),
		`SELECT type, name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		ORDER BY type, name`)
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var b strings.Builder

	for rows.Next() {
		var typ, name, stmt string
		if err := rows.Scan(&typ, &name, &stmt); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}

		fmt.Fprintf(&b, "-- %s %s\n%s;\n\n", typ, name, strings.TrimSpace(schemaPunctuation.ReplaceAllString(schemaWhitespace.ReplaceAllString(stmt, " "), "$1")))
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}

	return b.String()
}

func readSchemaGolden(t *testing.T) string {
	t.Helper()

	b, err := os.ReadFile(schemaGoldenPath)
	if err != nil {
		t.Fatalf("read %s: %v; run go test ./internal/db -run TestSchemaGolden -update", schemaGoldenPath, err)
	}

	return string(b)
}

func tableRowCounts(t *testing.T, conn *sql.DB) map[string]int {
	t.Helper()

	rows, err := conn.QueryContext(t.Context(), "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}

	var tables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}

		tables = append(tables, name)
	}

	_ = rows.Close()

	sort.Strings(tables)

	counts := make(map[string]int, len(tables))

	for _, table := range tables {
		var n int
		if err := conn.QueryRowContext(t.Context(), fmt.Sprintf("SELECT COUNT(*) FROM %q", table)).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}

		counts[table] = n
	}

	return counts
}

func schemaVersionOf(t *testing.T, conn *sql.DB) int {
	t.Helper()

	var v int
	if err := conn.QueryRowContext(t.Context(), "SELECT version FROM schema_version WHERE id = 1").Scan(&v); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}

	return v
}

func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	return conn
}

func extractReleaseFixture(t *testing.T, version string) string {
	t.Helper()

	f, err := os.Open(filepath.Join(releaseFixtureDir, version+".tar.gz"))
	if err != nil {
		t.Fatalf("open %s fixture: %v", version, err)
	}

	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("read %s fixture: %v", version, err)
	}

	dir := t.TempDir()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("read %s fixture: %v", version, err)
		}

		target := filepath.Join(dir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
			t.Fatalf("fixture entry %q escapes the extraction directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				t.Fatalf("mkdir %s: %v", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
			}

			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				t.Fatalf("create %s: %v", target, err)
			}

			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
				_ = out.Close()

				t.Fatalf("extract %s: %v", target, err)
			}

			if err := out.Close(); err != nil {
				t.Fatalf("close %s: %v", target, err)
			}
		}
	}

	return dir
}
