// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"
)

func TestIMSDataNetworkSessionIsNATExemptAndBindsAsIMS(t *testing.T) {
	for _, ims := range []bool{false, true} {
		store, upf := epsTestSMF()
		policy := epsPolicy()
		policy.IMS = ims
		pcf := &fakePCF{policy: policy}

		s := newTestSMF(pcf, store, upf, &fakeAMF{})

		if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err != nil {
			t.Fatal(err)
		}

		if upf.lastEstablish == nil || upf.lastEstablish.NATExempt != ims || upf.lastEstablish.LocalSwitch != ims {
			t.Fatalf("ims=%t: establish request %+v, want NAT exempt and local switch %t", ims, upf.lastEstablish, ims)
		}

		if len(pcf.associations) != 1 {
			t.Fatalf("ims=%t: %d policy associations, want 1", ims, len(pcf.associations))
		}

		for _, c := range pcf.associations {
			if c.IMS != ims {
				t.Fatalf("ims=%t: policy context IMS %t", ims, c.IMS)
			}
		}
	}
}
