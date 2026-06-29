// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// testSpanRecorder collects every span emitted during the test run. The global
// TracerProvider is set once, in TestMain: OpenTelemetry binds the kernel's
// `var Tracer = otel.Tracer(...)` to the first provider set in a process and never
// re-binds it, so a per-test SetTracerProvider only takes effect on the first run
// and breaks under `go test -count=N` (N>1). Setting it once here keeps tracing
// assertions stable across repeated and `-race` runs.
var testSpanRecorder = tracetest.NewSpanRecorder()

func TestMain(m *testing.M) {
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(testSpanRecorder)))

	os.Exit(m.Run())
}
