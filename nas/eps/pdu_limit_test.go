// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestAppendBinaryEnforcesPDULimit checks the cap on the appending primitive and
// not only on MarshalBinary. AppendBinary is the composable entry point
// (encoding.BinaryAppender), so a caller assembling a container reaches it
// directly, and a message no container could carry has to be refused there too.
//
// The limit is on what the message appends, never on the buffer it appends to,
// so each case runs against a non-empty prefix as well: a caller whose buffer
// already holds something must not see its own octets counted against the
// message (TS 24.301 §9, nas.MaxPDULen).
func TestAppendBinaryEnforcesPDULimit(t *testing.T) {
	// The SERVICE REQUEST is absent: TS 24.301 §8.2.25 fixes it at four octets
	// with no variable part, so no value of it can reach the limit.
	msgs := []Message{
		&ActivateDefaultEPSBearerContextAccept{},
		&ActivateDefaultEPSBearerContextReject{},
		&ActivateDefaultEPSBearerContextRequest{AccessPointName: APN("internet")},
		&AttachAccept{TAIList: TAIList{{Type: PartialTAIListConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}}}}},
		&AttachComplete{},
		&AttachReject{},
		&AttachRequest{EPSMobileIdentity: IMSIIdentity(IMSI("001010000000001"))},
		&AuthenticationFailure{},
		&AuthenticationReject{},
		&AuthenticationRequest{},
		&AuthenticationResponse{},
		&BearerResourceAllocationReject{},
		&BearerResourceAllocationRequest{},
		&BearerResourceModificationReject{},
		&BearerResourceModificationRequest{},
		&DeactivateEPSBearerContextAccept{},
		&DeactivateEPSBearerContextRequest{},
		&DetachAccept{},
		&DetachRequestNetwork{},
		&DetachRequestUE{EPSMobileIdentity: IMSIIdentity(IMSI("001010000000001"))},
		&EMMInformation{},
		&EMMStatus{},
		&ESMInformationRequest{},
		&ESMInformationResponse{},
		&ESMStatus{},
		&GUTIReallocationCommand{GUTI: IMSIIdentity(IMSI("001010000000001"))},
		&GUTIReallocationComplete{},
		&IdentityRequest{},
		&IdentityResponse{},
		&ModifyEPSBearerContextAccept{},
		&ModifyEPSBearerContextReject{},
		&ModifyEPSBearerContextRequest{},
		&PDNConnectivityReject{},
		&PDNConnectivityRequest{},
		&PDNDisconnectReject{},
		&PDNDisconnectRequest{},
		&SecurityModeCommand{},
		&SecurityModeComplete{},
		&SecurityModeReject{},
		&ServiceAccept{},
		&ServiceReject{},
		&TrackingAreaUpdateAccept{},
		&TrackingAreaUpdateComplete{},
		&TrackingAreaUpdateReject{},
		&TrackingAreaUpdateRequest{OldGUTI: IMSIIdentity(IMSI("001010000000001"))},
	}

	// The count ties the list to the dispatch tables so a new message has to be
	// added here too. Three modelled messages are absent from those tables — the
	// two DETACH REQUEST directions share one entry, and the SERVICE REQUEST is
	// reached through its security header type — and of those only the SERVICE
	// REQUEST is absent from the list, for the reason above.
	if want := len(emmParsers) + len(esmParsers) + 2; len(msgs) != want {
		t.Fatalf("%d messages listed, %d modelled: a new message needs a case here", len(msgs), want)
	}

	for _, m := range msgs {
		t.Run(reflect.TypeOf(m).Elem().Name(), func(t *testing.T) {
			oversize(t, m)

			for _, prefix := range [][]byte{nil, {0xAA, 0xBB, 0xCC}} {
				out, err := m.AppendBinary(prefix)
				if !errors.Is(err, nas.ErrPDUTooLong) {
					t.Fatalf("AppendBinary onto a %d-octet prefix: err = %v, want the PDU limit", len(prefix), err)
				}

				if !reflect.DeepEqual(out, prefix) {
					t.Errorf("the caller's buffer came back as % x, want % x", out, prefix)
				}
			}
		})
	}
}

// oversize attaches a preserved element that pushes a message past what any NAS
// container can carry. Every message models one, which is what makes this
// reachable for all of them.
func oversize(t *testing.T, m Message) {
	t.Helper()

	field := reflect.ValueOf(m).Elem().FieldByName("Unrecognized")
	if !field.IsValid() || !field.CanSet() {
		t.Fatalf("%T has no Unrecognized field, so this case cannot reach the limit", m)
	}

	field.Set(reflect.ValueOf([]nas.RawIE{
		{IEI: 0x7B, Format: nas.IETLVE, Value: make([]byte, nas.MaxPDULen)},
	}))
}
