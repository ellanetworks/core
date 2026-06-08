package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Reporter tracks test execution and produces a summary report.
type Reporter struct {
	mu    sync.Mutex
	tests map[string]*TestResult
	order []string // preserves insertion order
}

// TestResult holds the result of a single test or subtest.
type TestResult struct {
	Name      string
	Status    string // "pass", "fail", "skip"
	Reason    string // failure reason (empty if passed)
	Duration  time.Duration
	Logs      []string // captured quiet log lines
	StartTime time.Time
	EndTime   time.Time
}

// NewReporter creates a new test reporter.
func NewReporter() *Reporter {
	return &Reporter{
		tests: make(map[string]*TestResult),
	}
}

// Start records the beginning of a test. Returns the TestResult to track.
func (r *Reporter) Start(name string) *TestResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	tr := &TestResult{
		Name:      name,
		StartTime: time.Now(),
	}
	r.tests[name] = tr
	r.order = append(r.order, name)

	return tr
}

// Pass records a passing test.
func (r *Reporter) Pass(tr *TestResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tr.Status = "pass"
	tr.EndTime = time.Now()
	tr.Duration = tr.EndTime.Sub(tr.StartTime)
}

// Fail records a failing test with a reason.
func (r *Reporter) Fail(tr *TestResult, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tr.Status = "fail"
	tr.Reason = reason
	tr.EndTime = time.Now()
	tr.Duration = tr.EndTime.Sub(tr.StartTime)
}

// Skip records a skipped test with a reason.
func (r *Reporter) Skip(tr *TestResult, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tr.Status = "skip"
	tr.Reason = reason
	tr.EndTime = time.Now()
	tr.Duration = tr.EndTime.Sub(tr.StartTime)
}

// Log records a quiet log line for a test.
func (r *Reporter) Log(tr *TestResult, msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tr.Logs = append(tr.Logs, msg)
}

// FailureCount returns the number of failed tests.
func (r *Reporter) FailureCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int

	for _, tr := range r.tests {
		if tr.Status == "fail" {
			count++
		}
	}

	return count
}

// Range calls fn for each tracked test in insertion order.
// fn receives the test name and a pointer to its TestResult.
func (r *Reporter) Range(fn func(name string, tr *TestResult)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, name := range r.order {
		fn(name, r.tests[name])
	}
}

// Summary produces a compact summary string of all tracked tests.
func (r *Reporter) Summary() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var passCount, failCount, skipCount int

	for _, name := range r.order {
		tr := r.tests[name]
		switch tr.Status {
		case "pass":
			passCount++
		case "fail":
			failCount++
		case "skip":
			skipCount++
		}
	}

	var buf fmt.Stringer = &summaryBuilder{
		pass:   passCount,
		fail:   failCount,
		skip:   skipCount,
		orders: r.order,
		tests:  r.tests,
	}

	return buf.String()
}

type summaryBuilder struct {
	pass   int
	fail   int
	skip   int
	orders []string
	tests  map[string]*TestResult
}

func (b *summaryBuilder) String() string {
	total := b.pass + b.fail + b.skip

	var result string
	if b.fail > 0 {
		result = fmt.Sprintf("FAIL: %d total — %d failed, %d passed, %d skipped",
			total, b.fail, b.pass, b.skip)
	} else {
		result = fmt.Sprintf("PASS: %d total — %d passed, %d skipped",
			total, b.pass, b.skip)
	}

	// List failures with reasons
	if b.fail > 0 {
		result += "\n\nFailed tests:"

		for _, name := range b.orders {
			tr := b.tests[name]
			if tr.Status == "fail" {
				result += fmt.Sprintf("\n  - %s (%.1fs): %s",
					tr.Name, tr.Duration.Seconds(), tr.Reason)
			}
		}
	}

	return result
}

// globalReporter is the shared test reporter for all integration tests.
var globalReporter = NewReporter()

// setFailureReason records the failure reason on the TestResult.
// Call this from within the test before calling t.Fatalf.
func setFailureReason(tr *TestResult, reason string) {
	tr.Reason = reason
}

// registerScenarioTest creates a reporter entry for a scenario subtest.
// Call this before t.Run to track the scenario's execution.
func registerScenarioTest(scenarioName string) *TestResult {
	return globalReporter.Start(scenarioName)
}

// finishScenarioTest records the outcome of a scenario subtest.
// Call this after t.Run returns, using t.Failed() to determine pass/fail.
func finishScenarioTest(t *testing.T, tr *TestResult) {
	if t.Failed() {
		reason := tr.Reason
		if reason == "" {
			reason = "test failed"
		}

		globalReporter.Fail(tr, reason)
	} else {
		globalReporter.Pass(tr)
	}
}

// printTesterSummary prints a compact summary of the core-tester scenarios.
func printTesterSummary(t *testing.T) {
	summary := globalReporter.Summary()
	fmt.Println(summary)

	if globalReporter.FailureCount() > 0 {
		writeFailureReports(t, "core-tester")
	}
}

// isQuiet reports whether quiet reporting is enabled.
// QUIET_REPORT=false disables quiet mode (shows all logs).
func isQuiet() bool {
	return os.Getenv("QUIET_REPORT") != "false"
}

// QuietLog records a log line for the test without printing it.
// If quiet mode is disabled (QUIET_REPORT=false), prints immediately.
func QuietLog(t *testing.T, tr *TestResult, msg string) {
	if isQuiet() {
		globalReporter.Log(tr, msg)
	} else {
		t.Log(msg)
	}
}

// QuietLogf records a formatted log line for the test without printing it.
// If quiet mode is disabled (QUIET_REPORT=false), prints immediately.
func QuietLogf(t *testing.T, tr *TestResult, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	QuietLog(t, tr, msg)
}

// VerboseLog prints a log message immediately regardless of quiet mode.
// Use for critical messages that should always be visible.
func VerboseLog(t *testing.T, msg string) {
	t.Log(msg)
}

// VerboseLogf prints a formatted log message immediately regardless of quiet mode.
// Use for critical messages that should always be visible.
func VerboseLogf(t *testing.T, format string, args ...interface{}) {
	VerboseLog(t, fmt.Sprintf(format, args...))
}

// halogBuffer stores captured log lines for HA tests by test name.
var (
	halogBuffer = make(map[string][]string)
	halogMu     sync.Mutex
)

// HALog logs a message for an HA test. In quiet mode, the message is
// captured and only printed if the test fails. In verbose mode (or on
// failure), it is printed immediately.
func HALog(t *testing.T, msg string) {
	name := t.Name()

	if isQuiet() {
		halogMu.Lock()

		halogBuffer[name] = append(halogBuffer[name], msg)
		halogMu.Unlock()
	} else {
		t.Log(msg)
	}
}

// HALogf is the formatted version of HALog.
func HALogf(t *testing.T, format string, args ...interface{}) {
	HALog(t, fmt.Sprintf(format, args...))
}

// printHALogs prints captured HA test logs for failed tests.
// Should be called from a t.Cleanup after the test completes.
func printHALogs(t *testing.T) {
	name := t.Name()

	halogMu.Lock()
	logs, ok := halogBuffer[name]
	halogMu.Unlock()

	if !ok || len(logs) == 0 {
		return
	}

	// Only print logs for failed tests.
	if !t.Failed() {
		return
	}

	VerboseLogf(t, "=== %s: captured logs (%d lines) ===", name, len(logs))
	// Print last 30 lines to keep output manageable.
	start := 0
	if len(logs) > 30 {
		start = len(logs) - 30
	}

	for _, line := range logs[start:] {
		VerboseLog(t, line)
	}

	VerboseLog(t, "=== end captured logs ===")
}

// beginHATest registers a t.Cleanup that prints captured logs for failed HA tests.
// Call this at the start of each HA test function.
func beginHATest(t *testing.T) {
	t.Cleanup(func() {
		printHALogs(t)
	})
}

// writeFailureReports writes detailed failure reports to files.
func writeFailureReports(t *testing.T, testPrefix string) {
	logDir := os.Getenv("HA_CLUSTER_LOG_DIR")
	if logDir == "" {
		logDir = filepath.Join(os.TempDir(), "integration-failures")
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Logf("failed to create report dir %s: %v", logDir, err)
		return
	}

	globalReporter.Range(func(name string, tr *TestResult) {
		if tr.Status != "fail" {
			return
		}

		// Build filename from scenario name, replacing / with _.
		safeName := strings.ReplaceAll(name, "/", "_")
		filename := testPrefix + "_" + safeName + ".txt"
		path := filepath.Clean(filepath.Join(logDir, filename))

		if dir := filepath.Dir(path); dir != logDir {
			t.Logf("skipping report for %s: path escapes log directory", name)
			return
		}

		f, err := os.Create(path)
		if err != nil {
			t.Logf("failed to create report file %s: %v", path, err)
			return
		}

		_, _ = fmt.Fprintf(f, "=== %s ===\n", name)
		_, _ = fmt.Fprintf(f, "Duration: %.1fs\n", tr.Duration.Seconds())
		_, _ = fmt.Fprintf(f, "Error: %s\n", tr.Reason)
		_, _ = fmt.Fprintf(f, "\n--- Captured Logs ---\n")

		for _, line := range tr.Logs {
			_, _ = fmt.Fprintf(f, "%s\n", line)
		}

		_ = f.Close()
	})
}
