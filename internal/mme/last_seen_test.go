// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"
)

func TestLastSeenStore(t *testing.T) {
	var (
		older = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		newer = time.Date(2026, 8, 17, 10, 5, 0, 0, time.UTC)
		imsi  = "001010000000001"
	)

	t.Run("an unknown subscriber has no record", func(t *testing.T) {
		var store lastSeenStore

		if _, ok := store.get(imsi); ok {
			t.Error("get on an empty store reported a record")
		}
	})

	t.Run("an empty radio keeps the one already stored", func(t *testing.T) {
		var store lastSeenStore

		store.record(imsi, "id-enb-01", "enb-01", older)
		store.record(imsi, "", "", newer)

		got, ok := store.get(imsi)
		if !ok {
			t.Fatal("record did not store the subscriber")
		}

		if got.RadioID != "id-enb-01" || got.RadioName != "enb-01" {
			t.Errorf("radio = %q/%q, want it retained as id-enb-01/enb-01", got.RadioID, got.RadioName)
		}

		if !got.At.Equal(newer) {
			t.Errorf("At = %v, want %v", got.At, newer)
		}
	})

	t.Run("an older timestamp does not rewind the record", func(t *testing.T) {
		var store lastSeenStore

		store.record(imsi, "id-enb-01", "enb-01", newer)
		store.record(imsi, "id-enb-02", "enb-02", older)

		got, _ := store.get(imsi)
		if !got.At.Equal(newer) {
			t.Errorf("At = %v, want %v", got.At, newer)
		}

		if got.RadioID != "id-enb-02" || got.RadioName != "enb-02" {
			t.Errorf("radio = %q/%q, want id-enb-02/enb-02", got.RadioID, got.RadioName)
		}
	})

	t.Run("forget drops the record", func(t *testing.T) {
		var store lastSeenStore

		store.record(imsi, "id-enb-01", "enb-01", older)
		store.forget(imsi)

		if _, ok := store.get(imsi); ok {
			t.Error("forget left the record in place")
		}
	})

	t.Run("an empty IMSI is not stored", func(t *testing.T) {
		var store lastSeenStore

		store.record("", "id-enb-01", "enb-01", older)

		if len(store.all()) != 0 {
			t.Errorf("all() = %v, want empty", store.all())
		}
	})

	t.Run("all returns a copy", func(t *testing.T) {
		var store lastSeenStore

		store.record(imsi, "id-enb-01", "enb-01", older)

		snapshot := store.all()
		snapshot[imsi] = LastSeen{RadioName: "tampered"}

		got, _ := store.get(imsi)
		if got.RadioName != "enb-01" {
			t.Errorf("RadioName = %q, want the store unaffected at %q", got.RadioName, "enb-01")
		}
	})
}
