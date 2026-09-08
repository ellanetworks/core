// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func collectionErrorCount(t *testing.T, collector string) float64 {
	t.Helper()

	var m dto.Metric
	if err := collectionErrors.WithLabelValues(collector).Write(&m); err != nil {
		t.Fatalf("read collection error counter for %q: %v", collector, err)
	}

	return m.GetCounter().GetValue()
}

func TestCollectionErrorCountsPerCollector(t *testing.T) {
	before := collectionErrorCount(t, CollectorDatabaseIPAllocated)
	otherBefore := collectionErrorCount(t, CollectorUPFRingbuf)

	CollectionError(CollectorDatabaseIPAllocated)
	CollectionError(CollectorDatabaseIPAllocated)

	if got := collectionErrorCount(t, CollectorDatabaseIPAllocated) - before; got != 2 {
		t.Errorf("collection errors for %q = %v, want 2", CollectorDatabaseIPAllocated, got)
	}

	if got := collectionErrorCount(t, CollectorUPFRingbuf) - otherBefore; got != 0 {
		t.Errorf("collection errors for %q = %v, want 0", CollectorUPFRingbuf, got)
	}
}
