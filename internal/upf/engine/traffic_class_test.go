// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"strconv"
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestTrafficClassIDsAreStableAndDistinct(t *testing.T) {
	classes := NewTrafficClasses()

	cctv := TrafficClass{SST: 1, SD: "102030", DNN: "cctv", FiveQI: 2}
	control := TrafficClass{SST: 1, SD: "102030", DNN: "control", FiveQI: 82}

	a := classes.ID(cctv)
	b := classes.ID(control)

	if a == 0 || b == 0 {
		t.Fatalf("IDs = %d, %d; 0 is reserved for unclassified traffic", a, b)
	}

	if a == b {
		t.Fatalf("two classes share ID %d", a)
	}

	if again := classes.ID(cctv); again != a {
		t.Errorf("ID changed from %d to %d for the same class", a, again)
	}

	all := classes.All()
	if len(all) != 2 || all[a] != cctv || all[b] != control {
		t.Errorf("All() = %v, want {%d: %v, %d: %v}", all, a, cctv, b, control)
	}
}

func TestTrafficClassTableFull(t *testing.T) {
	classes := NewTrafficClasses()

	for i := 1; i < ebpf.MaxTrafficClasses; i++ {
		if id := classes.ID(TrafficClass{DNN: strconv.Itoa(i)}); id != uint16(i) {
			t.Fatalf("class %d got ID %d", i, id)
		}
	}

	if id := classes.ID(TrafficClass{DNN: "one-too-many"}); id != 0 {
		t.Errorf("ID past capacity = %d, want 0", id)
	}

	if got := len(classes.All()); got != ebpf.MaxTrafficClasses-1 {
		t.Errorf("All() holds %d classes, want %d", got, ebpf.MaxTrafficClasses-1)
	}
}

func TestSessionWithoutDataNetworkIsUnclassified(t *testing.T) {
	sess := NewSession(1)

	if _, ok := sess.trafficClass(9); ok {
		t.Error("a session with no data network produced a traffic class")
	}

	sess.SetTrafficClass(nil, "internet")

	c, ok := sess.trafficClass(9)
	if !ok || c != (TrafficClass{DNN: "internet", FiveQI: 9}) {
		t.Errorf("trafficClass = %+v, %v; want the data network with no slice", c, ok)
	}
}
