// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas/fgs"
)

func (f *roamingFixture) homeAuthentications(t *testing.T) []wire.AuthenticationInfo {
	t.Helper()

	var infos []wire.AuthenticationInfo

	for _, r := range f.home.Requests() {
		if r.Method != http.MethodPost || r.Path != wire.UEAuthenticationsPath() {
			continue
		}

		info, err := wire.Decode[wire.AuthenticationInfo](r.Body)
		if err != nil {
			t.Fatalf("decode the Nausf_UEAuthentication_Authenticate request: %v", err)
		}

		infos = append(infos, info)
	}

	return infos
}

// TS 33.501 §6.1.2
func TestVisitorRegisteredBySUCIAuthenticatesAtHomeWithTheSUCI(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)
	conn.SetIdentityTypeUsedForRegistration(uint8(fgs.IdentitySUCI))

	f.authenticate(t, ue, conn)

	infos := f.homeAuthentications(t)
	if len(infos) != 1 || infos[0].SupiOrSuci != roamerSUCI {
		t.Fatalf("home authentications %+v, want one with the SUCI %s", infos, roamerSUCI)
	}
}

// TS 33.501 §6.1.2
func TestVisitorReauthenticatedOnItsGUTIAuthenticatesAtHomeWithTheSUPI(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)
	conn.SetIdentityTypeUsedForRegistration(uint8(fgs.IdentitySUCI))

	f.authenticate(t, ue, conn)

	conn.SetIdentityTypeUsedForRegistration(uint8(fgs.IdentityGUTI))

	f.authenticate(t, ue, conn)

	infos := f.homeAuthentications(t)
	if len(infos) != 2 || infos[0].SupiOrSuci != roamerSUCI || infos[1].SupiOrSuci != "imsi-"+roamerIMSI {
		t.Fatalf("home authentications %+v, want the SUCI and then the SUPI", infos)
	}
}

// TS 29.509 §6.1.6.2.2
func TestVisitorResynchronisationSendsTheChallengeRAND(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)
	conn.SetIdentityTypeUsedForRegistration(uint8(fgs.IdentitySUCI))

	challenges := make(chan *ausf.AuthResult, 1)

	f.amf.RequestAuthentication(t.Context(), ue, conn, nil, func(_ context.Context, r *ausf.AuthResult, _ error) { challenges <- r })

	challenge := waitDone(t, challenges)

	done := make(chan error, 1)
	resync := &ausf.ResyncInfo{Rand: challenge.Rand, Auts: "0102030405060708090a0b0c0d0e"}

	f.amf.RequestAuthentication(t.Context(), ue, conn, resync, func(_ context.Context, _ *ausf.AuthResult, err error) { done <- err })

	if err := waitDone(t, done); err == nil {
		t.Fatal("the home network accepted a forged AUTS")
	}

	infos := f.homeAuthentications(t)
	if len(infos) != 2 || infos[1].ResynchronizationInfo == nil ||
		infos[1].ResynchronizationInfo.Rand != challenge.Rand || infos[1].ResynchronizationInfo.Auts != resync.Auts || infos[1].SupiOrSuci != roamerSUCI {
		t.Fatalf("home authentications %+v, want the re-synchronisation with the RAND %s", infos, challenge.Rand)
	}
}
