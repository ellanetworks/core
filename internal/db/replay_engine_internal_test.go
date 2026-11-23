// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

const replayModeEnv = "ELLA_DB_REPLAY"

var replayExclusions = map[string]string{
	"TestApplyEntriesHaltsOnAnUnclassifiedError":            "applies an operation registered only for the test",
	"TestApplyEntriesHaltsOnOperationAboveTheAppliedSchema": "applies an operation registered only for the test",
	"TestApplyEntriesHaltsOnUnknownOperation":               "applies an unregistered operation",
	"TestApplyEntriesIsolatesAFailingCommand":               "applies an operation registered only for the test",
	"TestApplyEntriesKeepsLeaderTimeMonotonic":              "applies an operation registered only for the test",
	"TestApplyEntriesRecoversFromAPanickingCommand":         "applies an operation registered only for the test",
	"TestApplyEntriesRunsForeignKeyCascades":                "applies an operation registered only for the test",
	"TestChangesetPathAcceptsPayloadsWithoutIDs":            "writes through the changeset path below the command schema",
	"TestChangesetPathKeepsTheOldSQNPayloadShape":           "writes through the changeset path below the command schema",
	"TestFSMAppliesInterleavedLegacyAndCommandEntries":      "applies legacy intent entries through the FSM between command entries",
	"TestReaderPoolServesReadsWhileWriterIsHeld":            "writes network_slices with direct SQL",
	"TestRegisterUE_RewritesRestoredRegistration":           "rewrites ue_registrations with direct SQL to simulate a restore",
	"TestReplicatedWritesApplyOnDivergedNode":               "diverges jwt_secret and cell_positions with direct SQL",
}

type replayPass struct {
	name       string
	groupSizes []int
	pragma     string
	physEvery  int
	timezone   string
	lockstep   bool
}

var (
	replaySinglePass = replayPass{name: "single", groupSizes: []int{1}}

	replayQuickPasses = []replayPass{
		{name: "combined", groupSizes: []int{1, 4, 2, 7, 3}, pragma: "PRAGMA reverse_unordered_selects = ON", physEvery: 16},
	}

	replayFullPasses = []replayPass{
		replaySinglePass,
		{name: "grouped", groupSizes: []int{1, 4, 2, 7, 3, 16}},
		{name: "reverse_unordered_selects", groupSizes: []int{1}, pragma: "PRAGMA reverse_unordered_selects = ON"},
		{name: "physical_order", groupSizes: []int{1}, physEvery: 1, lockstep: true},
		{name: "timezone", groupSizes: []int{1}, timezone: "Pacific/Chatham"},
	}
)

func replayPasses() ([]replayPass, error) {
	switch mode := os.Getenv(replayModeEnv); mode {
	case "", "quick":
		return replayQuickPasses, nil
	case "full":
		return replayFullPasses, nil
	case "off":
		return nil, nil
	default:
		return nil, fmt.Errorf("%s=%q: want quick, full or off", replayModeEnv, mode)
	}
}

func replayExclusionFor(test string) (string, bool) {
	for name := range replayExclusions {
		if strings.HasPrefix(test, sanitizeTestName(name)) {
			return name, true
		}
	}

	return "", false
}

func sanitizeTestName(name string) string {
	name = name[:min(len(name), 64)]

	return strings.Map(func(r rune) rune {
		if r == '/' {
			return -1
		}

		return r
	}, name)
}

func (r *replayRecorder) replay() []string {
	passes, err := replayPasses()
	if err != nil {
		return []string{err.Error()}
	}

	if len(passes) == 0 {
		return nil
	}

	var (
		mu       sync.Mutex
		failures []string
		used     = map[string]bool{}
	)

	fail := func(msg string) {
		mu.Lock()

		failures = append(failures, msg)
		mu.Unlock()
	}

	var replayable []*replayRecording

	for _, rec := range r.all() {
		if rec.skip != "" || len(rec.calls) == 0 {
			continue
		}

		if name, excluded := replayExclusionFor(rec.test); excluded {
			used[name] = true
			continue
		}

		if needs := rec.exclusionNeeds(); len(needs) > 0 {
			fail(fmt.Sprintf("%s (%s): %s\n    add it to replayExclusions with a reason if this is intended", rec.test, rec.db.dbPath, strings.Join(needs, "; ")))
			continue
		}

		replayable = append(replayable, rec)
	}

	for _, pass := range passes {
		if pass.timezone != "" {
			restore, err := setReplayTimezone(pass.timezone)
			if err != nil {
				fail(fmt.Sprintf("pass %s: %v", pass.name, err))
				continue
			}

			runReplayPass(r, pass, replayable, fail)
			restore()

			continue
		}

		runReplayPass(r, pass, replayable, fail)
	}

	if replayRanWholeSuite() && len(r.all()) > 0 {
		for name := range replayExclusions {
			if !used[name] {
				fail(fmt.Sprintf("replayExclusions[%q] matches no recorded database; remove the stale exclusion", name))
			}
		}
	}

	sort.Strings(failures)

	return failures
}

func replayRanWholeSuite() bool {
	for _, name := range []string{"test.run", "test.list", "test.skip", "test.fuzz"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return false
		}
	}

	return true
}

func (rec *replayRecording) exclusionNeeds() []string {
	var needs []string

	if ops := rec.unregisteredOperations(); len(ops) > 0 {
		needs = append(needs, "operations registered only during the test: "+strings.Join(ops, ", "))
	}

	return needs
}

func runReplayPass(r *replayRecorder, pass replayPass, recs []*replayRecording, fail func(string)) {
	work := make(chan *replayRecording)

	var wg sync.WaitGroup

	workers := runtime.GOMAXPROCS(0)
	if pass.timezone != "" {
		workers = 1
	}

	for range workers {
		wg.Go(func() {
			for rec := range work {
				if msg := replayRecordingOnce(r, pass, rec); msg != "" {
					fail(msg)
				}
			}
		})
	}

	for _, rec := range recs {
		work <- rec
	}

	close(work)
	wg.Wait()
}

func setReplayTimezone(name string) (func(), error) {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}

	previousTZ, hadTZ := os.LookupEnv("TZ")
	previousLocal := time.Local

	if err := os.Setenv("TZ", name); err != nil {
		return nil, err
	}

	time.Local = loc

	return func() {
		time.Local = previousLocal

		if hadTZ {
			_ = os.Setenv("TZ", previousTZ)
		} else {
			_ = os.Unsetenv("TZ")
		}
	}, nil
}

type replayStep struct {
	entries        []ellaraft.Entry
	results        []string
	err            string
	maxSubscribers int
	start          int
}

func (rec *replayRecording) steps(sizes []int) []replayStep {
	cuts := map[int]bool{}
	for _, cp := range rec.checkpoints {
		cuts[cp.applied] = true
	}

	var (
		steps []replayStep
		pos   int
		cycle int
	)

	for _, call := range rec.calls {
		if call.err != "" {
			steps = append(steps, replayStep{entries: call.entries, results: call.results, err: call.err, maxSubscribers: call.maxSubscribers, start: pos})
			pos += len(call.entries)

			continue
		}

		for i, e := range call.entries {
			n := len(steps)
			if n == 0 || steps[n-1].err != "" || len(steps[n-1].entries) >= sizes[cycle%len(sizes)] || cuts[pos] || steps[n-1].maxSubscribers != call.maxSubscribers {
				if n > 0 && steps[n-1].err == "" {
					cycle++
				}

				steps = append(steps, replayStep{maxSubscribers: call.maxSubscribers, start: pos})
				n++
			}

			steps[n-1].entries = append(steps[n-1].entries, e)
			steps[n-1].results = append(steps[n-1].results, call.results[i])
			pos++
		}
	}

	return steps
}

func replayRecordingOnce(r *replayRecorder, pass replayPass, rec *replayRecording) string {
	ctx := context.Background()
	label := fmt.Sprintf("replay %s of %s (%d entries)", pass.name, rec.test, rec.entries)

	replica, cleanup, err := openReplayReplica(ctx, r.dir, rec, pass)
	if err != nil {
		return fmt.Sprintf("%s: open replica: %v", label, err)
	}

	defer cleanup()

	checkpoints := map[int]stateCheckpoint{}
	for _, cp := range rec.checkpoints {
		checkpoints[cp.applied] = cp
	}

	if msg := compareCheckpoint(ctx, replica, rec, checkpoints, 0, label); msg != "" {
		return msg
	}

	sinceReorder := 0

	var baseline *Database

	if pass.lockstep {
		b, cleanBaseline, err := openReplayReplica(ctx, r.dir, rec, replaySinglePass)
		if err != nil {
			return fmt.Sprintf("%s: open baseline replica: %v", label, err)
		}

		defer cleanBaseline()

		baseline = b
	}

	for _, step := range rec.steps(pass.groupSizes) {
		if pass.physEvery > 0 && sinceReorder >= pass.physEvery {
			if err := replayReversePhysicalOrder(ctx, replica); err != nil {
				return fmt.Sprintf("%s: reverse physical order: %v", label, err)
			}

			sinceReorder = 0
		}

		replica.maxSubscribers = step.maxSubscribers

		out, err := replica.applyEntries(ctx, step.entries)

		errText := ""
		if err != nil {
			errText = err.Error()
		}

		if errText != step.err {
			return fmt.Sprintf("%s: entries %d..%d: apply error %q, recorded %q\n    operations: %s", label, step.entries[0].Index, step.entries[len(step.entries)-1].Index, errText, step.err, strings.Join(rec.operations(step.start, step.start+len(step.entries)), ", "))
		}

		if err == nil {
			got := encodeApplyResults(out)

			for i := range step.entries {
				if got[i] != step.results[i] {
					return fmt.Sprintf("%s: entry %d (%s): result differs\n    recorded: %s\n    replayed: %s", label, step.entries[i].Index, strings.Join(rec.operations(step.start+i, step.start+i+1), ", "), step.results[i], got[i])
				}
			}
		}

		sinceReorder += len(step.entries)

		if baseline != nil && step.start < replayLockstepEntries && step.err == "" {
			if msg := compareWithBaseline(ctx, baseline, replica, rec, step, label); msg != "" {
				return msg
			}
		}

		if msg := compareCheckpoint(ctx, replica, rec, checkpoints, step.start+len(step.entries), label); msg != "" {
			return msg + attributeDivergence(ctx, r, pass, rec, step.start+len(step.entries))
		}
	}

	return ""
}

const replayLockstepEntries = 64

func compareWithBaseline(ctx context.Context, baseline, replica *Database, rec *replayRecording, step replayStep, label string) string {
	baseline.maxSubscribers = step.maxSubscribers

	for _, e := range step.entries {
		if _, err := baseline.applyEntries(ctx, []ellaraft.Entry{e}); err != nil {
			return fmt.Sprintf("%s: baseline replica failed at entry %d: %v", label, e.Index, err)
		}
	}

	want, err := replicatedTableHashes(ctx, baseline.writer())
	if err != nil {
		return fmt.Sprintf("%s: hash baseline: %v", label, err)
	}

	got, err := replicatedTableHashes(ctx, replica.writer())
	if err != nil {
		return fmt.Sprintf("%s: hash replica: %v", label, err)
	}

	var differ []string

	for table, h := range want {
		if got[table] != h {
			differ = append(differ, table)
		}
	}

	if len(differ) == 0 {
		return ""
	}

	sort.Strings(differ)

	return fmt.Sprintf("%s: replicated tables %s differ from an unperturbed replay after entries %d..%d: %s",
		label, strings.Join(differ, ", "), step.entries[0].Index, step.entries[len(step.entries)-1].Index,
		strings.Join(rec.operations(step.start, step.start+len(step.entries)), ", "))
}

func compareCheckpoint(ctx context.Context, replica *Database, rec *replayRecording, checkpoints map[int]stateCheckpoint, applied int, label string) string {
	cp, ok := checkpoints[applied]
	if !ok {
		return ""
	}

	got, err := replicatedTableHashes(ctx, replica.writer())
	if err != nil {
		return fmt.Sprintf("%s: hash replica: %v", label, err)
	}

	var differ []string

	for table, want := range cp.tables {
		if got[table] != want {
			differ = append(differ, table)
		}
	}

	if len(differ) == 0 {
		return ""
	}

	sort.Strings(differ)

	previous := 0

	for _, other := range rec.checkpoints {
		if other.applied < applied {
			previous = other.applied
		}
	}

	if cp.external {
		return fmt.Sprintf("%s: replicated tables %s were changed outside ApplyEntries after %d entries (direct SQL write or restore), or entries since %d diverged: %s\n    add the test to replayExclusions with a reason if it writes replicated tables directly",
			label, strings.Join(differ, ", "), applied, previous, lastOperations(rec.operations(previous, applied)))
	}

	return fmt.Sprintf("%s: replicated tables differ after %d entries: %s\n    operations since the last matching state: %s",
		label, applied, strings.Join(differ, ", "), lastOperations(rec.operations(previous, applied)))
}

func attributeDivergence(ctx context.Context, r *replayRecorder, pass replayPass, rec *replayRecording, upTo int) string {
	baseline, cleanBaseline, err := openReplayReplica(ctx, r.dir, rec, replaySinglePass)
	if err != nil {
		return ""
	}

	defer cleanBaseline()

	perturbed, cleanPerturbed, err := openReplayReplica(ctx, r.dir, rec, pass)
	if err != nil {
		return ""
	}

	defer cleanPerturbed()

	pos := 0

	for _, step := range rec.steps([]int{1}) {
		if pos >= upTo {
			break
		}

		baseline.maxSubscribers = step.maxSubscribers
		perturbed.maxSubscribers = step.maxSubscribers

		if pass.physEvery > 0 {
			if err := replayReversePhysicalOrder(ctx, perturbed); err != nil {
				return ""
			}
		}

		_, errA := baseline.applyEntries(ctx, step.entries)
		_, errB := perturbed.applyEntries(ctx, step.entries)

		a, hashErrA := replicatedTableHashes(ctx, baseline.writer())
		b, hashErrB := replicatedTableHashes(ctx, perturbed.writer())

		if hashErrA != nil || hashErrB != nil {
			return ""
		}

		if (errA == nil) != (errB == nil) || !sameTableHashes(a, b) {
			return fmt.Sprintf("\n    first entry that differs from a second unperturbed replay: %s", strings.Join(rec.operations(step.start, step.start+len(step.entries)), ", "))
		}

		pos += len(step.entries)
	}

	return ""
}

func lastOperations(ops []string) string {
	const shown = 12

	if len(ops) > shown {
		return fmt.Sprintf("... %d more, %s", len(ops)-shown, strings.Join(ops[len(ops)-shown:], ", "))
	}

	return strings.Join(ops, ", ")
}

func sameTableHashes(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}

var replicaSeq struct {
	mu sync.Mutex
	n  int
}

func openReplayReplica(ctx context.Context, root string, rec *replayRecording, pass replayPass) (*Database, func(), error) {
	replicaSeq.mu.Lock()
	replicaSeq.n++
	n := replicaSeq.n
	replicaSeq.mu.Unlock()

	dir := filepath.Join(root, fmt.Sprintf("replica-%06d", n))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}

	path := filepath.Join(dir, "db.sqlite3")

	if err := copyFile(rec.snapshot, path); err != nil {
		return nil, nil, err
	}

	conn, connector, err := openSQLitePool(ctx, path)
	if err != nil {
		return nil, nil, err
	}

	d := new(Database)
	d.connection = conn
	d.connector = connector
	d.dbPath = path
	d.dataDir = dir
	d.changefeed = NewChangefeed()

	cleanup := func() {
		_ = d.Close()
		_ = os.RemoveAll(dir)
	}

	if err := d.openReaderPool(ctx); err != nil {
		cleanup()
		return nil, nil, err
	}

	if err := d.refreshAppliedSchema(ctx); err != nil {
		cleanup()
		return nil, nil, err
	}

	if err := d.loadAppliedLeaderTime(ctx); err != nil {
		cleanup()
		return nil, nil, err
	}

	if pass.pragma != "" {
		if _, err := d.writer().ExecContext(ctx, pass.pragma); err != nil {
			cleanup()
			return nil, nil, err
		}
	}

	if pass.physEvery > 0 {
		if err := replayReversePhysicalOrder(ctx, d); err != nil {
			cleanup()
			return nil, nil, err
		}
	}

	return d, cleanup, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}

func replayReversePhysicalOrder(ctx context.Context, d *Database) error {
	conn, err := d.writer().Conn(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}

	defer func() { _, _ = conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON") }()

	tables := slices.Sorted(slices.Values(replicatedChangesetTables))

	for _, t := range tables {
		var ddl string
		if err := conn.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", t).Scan(&ddl); err != nil {
			continue
		}

		if strings.Contains(strings.ToUpper(ddl), "WITHOUT ROWID") {
			continue
		}

		var n int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdentifier(t)).Scan(&n); err != nil || n < 2 {
			continue
		}

		q := quoteIdentifier(t)

		stmts := []string{
			"BEGIN IMMEDIATE",
			"CREATE TEMP TABLE replay_reversed AS SELECT * FROM " + q + " ORDER BY rowid DESC",
			"DELETE FROM " + q,
			"INSERT INTO " + q + " SELECT * FROM replay_reversed",
			"DROP TABLE replay_reversed",
			"COMMIT",
		}

		for _, st := range stmts {
			if _, err := conn.ExecContext(ctx, st); err != nil {
				_, _ = conn.ExecContext(ctx, "ROLLBACK")
				_, _ = conn.ExecContext(ctx, "DROP TABLE IF EXISTS replay_reversed")

				return fmt.Errorf("reverse %s: %s: %w", t, st, err)
			}
		}
	}

	return nil
}
