// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nasreply

import (
	"context"
	"testing"
)

type recordingEgress struct {
	mmStatus  []uint8
	smStatus  []uint8
	discarded []Reason
}

func (e *recordingEgress) SendMMStatus(_ context.Context, cause uint8) {
	e.mmStatus = append(e.mmStatus, cause)
}

func (e *recordingEgress) SendSMStatus(_ context.Context, cause uint8) {
	e.smStatus = append(e.smStatus, cause)
}

func (e *recordingEgress) Discard(_ context.Context, reason Reason) {
	e.discarded = append(e.discarded, reason)
}

func TestFinalize(t *testing.T) {
	tests := []struct {
		name        string
		disposition Disposition
		wantMM      []uint8
		wantSM      []uint8
		wantDiscard []Reason
	}{
		{
			name:        "handled sends nothing",
			disposition: Handled(),
		},
		{
			name:        "status MM sends a 5GMM/EMM STATUS",
			disposition: StatusMM(CauseMessageTypeNotImplemented),
			wantMM:      []uint8{97},
		},
		{
			name:        "status SM sends a 5GSM/ESM STATUS",
			disposition: StatusSM(CauseInvalidMandatoryInfo),
			wantSM:      []uint8{96},
		},
		{
			name:        "silent audits the reason, sends no STATUS",
			disposition: Silent(ReasonIntegrityFail),
			wantDiscard: []Reason{ReasonIntegrityFail},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &recordingEgress{}
			tc.disposition.Finalize(context.Background(), e)

			if !equalU8(e.mmStatus, tc.wantMM) {
				t.Errorf("MM STATUS = %v, want %v", e.mmStatus, tc.wantMM)
			}

			if !equalU8(e.smStatus, tc.wantSM) {
				t.Errorf("SM STATUS = %v, want %v", e.smStatus, tc.wantSM)
			}

			if len(e.discarded) != len(tc.wantDiscard) {
				t.Fatalf("discards = %v, want %v", e.discarded, tc.wantDiscard)
			}

			for i := range e.discarded {
				if e.discarded[i] != tc.wantDiscard[i] {
					t.Errorf("discard[%d] = %v, want %v", i, e.discarded[i], tc.wantDiscard[i])
				}
			}
		})
	}
}

func equalU8(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
