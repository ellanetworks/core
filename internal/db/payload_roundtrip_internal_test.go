// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

type payloadRoundTrips struct {
	mu       sync.Mutex
	invoked  map[string]int
	failures map[string]string
}

var roundTrips = &payloadRoundTrips{invoked: map[string]int{}, failures: map[string]string{}}

func (r *payloadRoundTrips) observe(op string, payload any) {
	failure := payloadRoundTripFailure(payload)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.invoked[op]++

	if failure != "" {
		if _, seen := r.failures[op]; !seen {
			r.failures[op] = failure
		}
	}
}

func (r *payloadRoundTrips) report() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]string, 0, len(r.failures))
	for op, failure := range r.failures {
		out = append(out, fmt.Sprintf("%s: %s", op, failure))
	}

	sort.Strings(out)

	return out
}

func payloadRoundTripFailure(payload any) string {
	if payload == nil {
		return ""
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("marshal: %v", err)
	}

	original := reflect.ValueOf(payload)

	decoded := reflect.New(original.Type())
	if err := json.Unmarshal(encoded, decoded.Interface()); err != nil {
		return fmt.Sprintf("unmarshal %s: %v", encoded, err)
	}

	if !payloadEqual(original, decoded.Elem()) {
		return fmt.Sprintf("%#v decodes from %s as %#v", payload, encoded, decoded.Elem().Interface())
	}

	return ""
}

var timeType = reflect.TypeFor[time.Time]()

func payloadEqual(a, b reflect.Value) bool {
	if a.IsValid() != b.IsValid() {
		return false
	}

	if !a.IsValid() {
		return true
	}

	if a.Type() != b.Type() {
		return false
	}

	if a.Type() == timeType {
		return a.Interface().(time.Time).Equal(b.Interface().(time.Time))
	}

	if a.Type() == reflect.TypeFor[json.RawMessage]() {
		return compactJSON(a.Bytes()) == compactJSON(b.Bytes())
	}

	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}

		return payloadEqual(a.Elem(), b.Elem())
	case reflect.Struct:
		for i := range a.NumField() {
			if !a.Type().Field(i).IsExported() {
				continue
			}

			if !payloadEqual(a.Field(i), b.Field(i)) {
				return false
			}
		}

		return true
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}

		for i := range a.Len() {
			if !payloadEqual(a.Index(i), b.Index(i)) {
				return false
			}
		}

		return true
	case reflect.Array:
		for i := range a.Len() {
			if !payloadEqual(a.Index(i), b.Index(i)) {
				return false
			}
		}

		return true
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}

		for _, k := range a.MapKeys() {
			if !payloadEqual(a.MapIndex(k), b.MapIndex(k)) {
				return false
			}
		}

		return true
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface())
	}
}

func compactJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}

	return buf.String()
}

var testRecorder *replayRecorder

func TestMain(m *testing.M) {
	observeInvoke = roundTrips.observe

	recorder, err := newReplayRecorder()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start replay recorder: %v\n", err)
		os.Exit(1)
	}

	testRecorder = recorder
	recorder.install()

	code := m.Run()

	recorder.finishAll()
	recorder.uninstall()

	start := time.Now()

	if failures := recorder.replay(); len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d replays do not reproduce the recorded state or results:\n", len(failures))

		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  "+f)
		}

		code = 1
	} else if testing.Verbose() {
		fmt.Fprintf(os.Stderr, "replayed %d recorded databases in %s\n", len(recorder.all()), time.Since(start))
	}

	recorder.cleanup()

	if failures := roundTrips.report(); len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d operation payloads do not survive a JSON round trip:\n", len(failures))

		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  "+f)
		}

		code = 1
	}

	os.Exit(code)
}

func TestPayloadRoundTripDetectsALossyPayload(t *testing.T) {
	type lossy struct {
		Kept    string   `json:"kept"`
		Dropped string   `json:"-"`
		Empty   []string `json:"empty,omitempty"`
	}

	if failure := payloadRoundTripFailure(&lossy{Kept: "a", Dropped: "b"}); failure == "" {
		t.Fatal("a field that JSON drops must fail the round trip")
	}

	if failure := payloadRoundTripFailure(&lossy{Kept: "a", Empty: []string{}}); failure == "" {
		t.Fatal("an empty slice omitted by JSON must fail the round trip")
	}

	if failure := payloadRoundTripFailure(&lossy{Kept: "a"}); failure != "" {
		t.Fatalf("a lossless payload failed the round trip: %s", failure)
	}

	if failure := payloadRoundTripFailure(&releaseIPLeasePayload{NodeID: "node-1"}); failure == "" {
		t.Fatal("an invalid node ID must fail the round trip")
	}
}
