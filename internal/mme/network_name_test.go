// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udm"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// spnBearerStore is a fakeBearerStore with a configured service provider name.
type spnBearerStore struct{ fakeBearerStore }

func (spnBearerStore) GetOperator(_ context.Context) (*db.Operator, error) {
	return &db.Operator{
		Mcc: "001", Mnc: "01", SupportedTACs: `["1"]`,
		Ciphering: `["AES"]`, Integrity: `["AES"]`,
		SpnFullName: "Ella", SpnShortName: "Ella",
	}, nil
}

// TestSendNetworkName checks that, with an SPN configured, the MME provides the
// network name to the UE in an integrity-protected EMM INFORMATION message
// (TS 24.301 §5.4.5).
func TestSendNetworkName(t *testing.T) {
	m := New(udm.New(newFakeCredStore(), noopKeyResolver), spnBearerStore{}, &fakeSessionManager{})
	ue, cc := securedUE(t, m)

	m.sendNetworkName(ue)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one EMM INFORMATION downlink, got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect EMM INFORMATION: %v", err)
	}

	if mt, err := eps.PeekMessageType(plain); err != nil || mt != eps.MsgEMMInformation {
		t.Fatalf("downlink message = %#x (err %v), want EMM INFORMATION", mt, err)
	}
}

// TestSendNetworkNameNoSPN checks that the optional procedure is skipped when no
// service provider name is configured.
func TestSendNetworkNameNoSPN(t *testing.T) {
	m := newTestMME(t) // fakeBearerStore has no SPN configured
	ue, cc := securedUE(t, m)

	m.sendNetworkName(ue)

	if len(cc.sent) != 0 {
		t.Fatalf("expected no downlink without an SPN, got %d", len(cc.sent))
	}
}
