// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"maps"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestQERValidationWithoutBPF(t *testing.T) {
	qers := make([]models.QER, ebpf.MaxSessionQERs)
	for i := range qers {
		qers[i].QERID = uint32(i + 1)
	}

	t.Run("establish", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			req  models.EstablishRequest
			want string
		}{
			{"at limit", models.EstablishRequest{SEID: 1, QERs: qers}, ""},
			{"over limit", models.EstablishRequest{SEID: 1, QERs: append(slices.Clone(qers), models.QER{QERID: 99})}, "QERs"},
			{"missing reference", models.EstablishRequest{SEID: 1, PDRs: []models.PDR{{PDRID: 2, QERID: 9, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.1")}}}}, "references QER 9"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				conn := newTestEngine()

				_, err := conn.EstablishSession(t.Context(), &tc.req)
				if tc.want != "" {
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("EstablishSession = %v, want %q", err, tc.want)
					}

					if conn.GetSession(1) != nil {
						t.Fatal("validation failure registered a session")
					}

					return
				}

				if err != nil {
					t.Fatal(err)
				}

				if got := len(conn.GetSession(1).qerMeters); got != ebpf.MaxSessionQERs {
					t.Fatalf("owned meters = %d, want %d", got, ebpf.MaxSessionQERs)
				}

				if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: 1}); err != nil {
					t.Fatal(err)
				}
			})
		}
	})

	t.Run("modify", func(t *testing.T) {
		conn := newTestEngine()
		if _, err := conn.EstablishSession(t.Context(), &models.EstablishRequest{SEID: 1, QERs: qers}); err != nil {
			t.Fatal(err)
		}

		session := conn.GetSession(1)
		session.PutPDR(2, SPDRInfo{PdrID: 2, PdrInfo: ebpf.PdrInfo{QerID: 1}})
		pdrs, fars, before := session.snapshot()
		owners := maps.Clone(session.qerMeters)

		for _, tc := range []struct {
			name string
			req  models.ModifyRequest
			want string
		}{
			{"removed reference", models.ModifyRequest{SEID: 1, RemoveQERs: []uint32{1}}, "still references removed QER 1"},
			{"missing reference", models.ModifyRequest{SEID: 1, UpdatePDRs: []models.PDR{{PDRID: 2, QERID: 99, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.1")}}}}, "references QER 99"},
			{"over limit", models.ModifyRequest{SEID: 1, UpdateQERs: []models.QER{{QERID: 99}}}, "QERs"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := conn.ModifySession(t.Context(), &tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("ModifySession = %v, want %q", err, tc.want)
				}

				gotPDRs, gotFARs, gotQERs := session.snapshot()
				if !reflect.DeepEqual(gotPDRs, pdrs) || !maps.Equal(gotFARs, fars) || !maps.Equal(gotQERs, before) || !maps.Equal(session.qerMeters, owners) {
					t.Fatal("validation failure changed session rules or meter ownership")
				}
			})
		}
	})

	session := NewSession(1)
	session.PutPDR(2, SPDRInfo{PdrID: 2})

	if err := checkSessionQERReferences(session); err != nil {
		t.Fatalf("QER ID zero must not require a meter: %v", err)
	}
}

func TestModifyRemovingAURRAndAQERWaitsOnce(t *testing.T) {
	conn := newTestEngine()

	req := &models.EstablishRequest{SEID: 1, QERs: []models.QER{{QERID: 1}, {QERID: 2}}, URRs: []models.URR{{URRID: 1}}}
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	session := conn.GetSession(1)
	session.unpublished = false

	waiter := &countingBarrier{}
	conn.programs = waiter

	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: 1, RemoveQERs: []uint32{2}, RemoveURRs: []uint32{1}}); err != nil {
		t.Fatal(err)
	}

	if waiter.calls != 1 {
		t.Fatalf("barriers = %d, want one covering both the URR and the QER meter", waiter.calls)
	}

	if _, owned := session.qerMeters[2]; owned || session.usage.tracks(1) {
		t.Fatal("the removed QER meter or URR was not cleaned up")
	}
}

type countingBarrier struct{ calls int }

func (b *countingBarrier) Wait() error {
	b.calls++
	return nil
}

func TestQERMeterOwnershipWithoutBPF(t *testing.T) {
	conn := newTestEngine()
	session := NewSession(1)
	session.PutQer(1, ebpf.QerInfo{})

	created, err := conn.createQERMeters(session, nil)
	if err != nil || !slices.Equal(created, []uint32{1}) {
		t.Fatalf("create = %v, %v", created, err)
	}

	if created, err := conn.createQERMeters(session, session.ListQERs()); err != nil || len(created) != 0 {
		t.Fatalf("unchanged QER allocated meters: %v, %v", created, err)
	}

	if _, err := conn.createQERMeters(session, nil); err == nil {
		t.Fatal("adopted an already-owned meter as new")
	}

	for range 2 {
		if err := conn.deleteQERMeters(session, []uint32{1, 99}); err != nil || len(session.qerMeters) != 0 {
			t.Fatalf("idempotent delete = %v, owners = %v", err, session.qerMeters)
		}
	}

	conn.BpfObjects = &ebpf.BpfObjects{}
	if _, err := conn.createQERMeters(session, nil); err == nil || !strings.Contains(err.Error(), "map is not initialized") {
		t.Fatalf("missing map create = %v", err)
	}

	if len(session.qerMeters) != 0 {
		t.Fatal("failed allocation acquired ownership")
	}

	session.qerMeters[1] = struct{}{}
	if err := conn.deleteQERMeters(session, []uint32{1}); err == nil {
		t.Fatal("missing map deletion succeeded")
	}

	if _, owned := session.qerMeters[1]; !owned {
		t.Fatal("failed deletion forgot ownership")
	}
}
