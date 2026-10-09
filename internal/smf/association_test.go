// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

type associationRecorder struct {
	created    []string
	terminated []string
}

func (r *associationRecorder) CreateAssociation(_ context.Context, ref string, _ PolicyContext) (*PolicyDecision, error) {
	r.created = append(r.created, ref)

	return &PolicyDecision{Revision: 1}, nil
}

func (r *associationRecorder) UpdateAssociation(context.Context, string, SubscribedQoS) (*PolicyDecision, error) {
	return &PolicyDecision{}, nil
}

func (r *associationRecorder) ReportEnforcementFailure(string, []RuleReport, EnforcementFailure) {
}

func (r *associationRecorder) TerminateAssociation(ref string) {
	r.terminated = append(r.terminated, ref)
}

func TestNoAssociationForASessionAlreadyDropped(t *testing.T) {
	pcf := &associationRecorder{}
	s := New(pcf, nil, nil, nil)

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	sc, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: 1}, "ims", nil)
	if err != nil {
		t.Fatal(err)
	}

	s.dropFromPool(sc)

	if _, err := s.createAssociation(context.Background(), sc, PolicyContext{Supi: supi, Dnn: "ims"}); err == nil || len(pcf.created) != 0 {
		t.Fatalf("createAssociation for a dropped session = %v, created %v; want an error and none", err, pcf.created)
	}

	live, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: 1}, "ims", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.createAssociation(context.Background(), live, PolicyContext{Supi: supi, Dnn: "ims"}); err != nil || len(pcf.created) != 1 || pcf.created[0] != live.Ref {
		t.Fatalf("createAssociation = %v, created %v; want [%s]", err, pcf.created, live.Ref)
	}
}
