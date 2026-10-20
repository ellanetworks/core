// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"slices"
	"testing"

	"github.com/cilium/ebpf/link"
)

type recordingLink struct {
	link.Link
	name   string
	closed *[]string
}

func (l recordingLink) Close() error {
	*l.closed = append(*l.closed, l.name)

	return nil
}

func TestStopDatapathDetachesEveryAccountingProgramOnce(t *testing.T) {
	var closed []string

	n6 := link.Link(recordingLink{name: "n6", closed: &closed})
	u := &UPF{
		n3Link:          recordingLink{name: "n3", closed: &closed},
		n6Link:          &n6,
		bufferResponder: &BufferResponder{vethLink: recordingLink{name: "buffer", closed: &closed}},
	}

	if err := u.StopDatapath(); err != nil {
		t.Fatalf("StopDatapath: %v", err)
	}

	if want := []string{"n6", "n3", "buffer"}; !slices.Equal(closed, want) {
		t.Fatalf("detached %v, want %v", closed, want)
	}

	if err := u.StopDatapath(); err != nil {
		t.Fatalf("second StopDatapath: %v", err)
	}

	if len(closed) != 3 {
		t.Fatalf("detached %v after a second stop, want each program detached once", closed)
	}
}
