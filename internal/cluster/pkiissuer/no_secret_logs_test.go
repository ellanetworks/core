// Copyright 2026 Ella Networks

package pkiissuer_test

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestIssuer_NoSecretBytesInLogs asserts that bootstrap and issuance
// don't emit PEM private-key markers or long base64 runs to the
// logger.
func TestIssuer_NoSecretBytesInLogs(t *testing.T) {
	var buf bytes.Buffer

	encoderCfg := zap.NewProductionEncoderConfig()
	encoder := zapcore.NewJSONEncoder(encoderCfg)
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.DebugLevel)
	testLogger := zap.New(core)

	orig := logger.RaftLog
	logger.RaftLog = testLogger

	t.Cleanup(func() { logger.RaftLog = orig })

	store := newFakeStore("cluster-secrets-test")
	svc := pkiissuer.New(store)
	ctx := context.Background()

	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Issue a leaf so the full logging surface of the issuer runs.
	_, csrPEM, err := pki.GenerateKeyAndCSR(7, "cluster-secrets-test")
	if err != nil {
		t.Fatal(err)
	}

	csr, err := pki.ParseCSRPEM(csrPEM)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Issue(ctx, csr, 7, time.Hour); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	logs := buf.String()

	for _, marker := range []string{
		"BEGIN PRIVATE KEY",
		"BEGIN RSA PRIVATE KEY",
		"BEGIN EC PRIVATE KEY",
		"BEGIN ENCRYPTED PRIVATE KEY",
	} {
		if strings.Contains(logs, marker) {
			t.Fatalf("log output contains %q: possible key-material leak", marker)
		}
	}

	// Long base64 runs (≥48 chars, typical PEM line width) are
	// suspicious. Expected public material in logs is limited to
	// fingerprints (sha256:...) and short IDs, so the heuristic gives
	// tight secret detection without false positives.
	longBase64 := regexp.MustCompile(`[A-Za-z0-9+/]{48,}={0,2}`)

	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "sha256:") || strings.Contains(line, "Fingerprint") {
			continue
		}

		if m := longBase64.FindString(line); m != "" {
			t.Fatalf("log output contains suspicious base64-like run %q in line: %s", m, line)
		}
	}
}
