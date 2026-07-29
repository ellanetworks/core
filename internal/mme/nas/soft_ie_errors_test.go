// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TestDecodedTreatsSoftErrorsAsUsable pins the §7.7.1 decision the handlers
// share: a message whose only failures are syntactically incorrect optional IEs
// is acted on, and anything else is refused.
func TestDecodedTreatsSoftErrorsAsUsable(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"no error":                  {nil, true},
		"one bad optional element":  {&nas.IEError{IEI: 0x5E, Err: nas.ErrTruncated}, true},
		"several bad optional ones": {errors.Join(&nas.IEError{IEI: 0x5E}, &nas.IEError{IEI: 0x50}), true},
		"bad framing":               {&nas.Error{Op: "LV", Err: nas.ErrTruncated}, false},
		"a security-critical one":   {errors.New("nas/eps: UE network capability is 3 octets"), false},
		"soft alongside hard":       {errors.Join(&nas.IEError{IEI: 0x5E}, nas.ErrTruncated), false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := decoded(context.Background(), "AttachRequest", tc.err); got != tc.want {
				t.Fatalf("decoded(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestAttachRequestWithMalformedOptionalIEStillAttaches is the end-to-end shape
// of TS 24.301 §7.7.1: a UE that sends an unusable optional element is not
// rejected, the element is absent, and the rest of the request is honoured.
func TestAttachRequestWithMalformedOptionalIEStillAttaches(t *testing.T) {
	esm, err := (&eps.PDNConnectivityRequest{
		PTI: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	base, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI("001010000000001")),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// An additional GUTI (IEI 0x50, TLV) that delimits cleanly but whose value is
	// two octets where an EPS mobile identity needs eleven.
	wire := append(base, 0x50, 0x02, 0xf6, 0x00)

	req, err := eps.ParseAttachRequest(wire)
	if err == nil || !nas.SoftOnly(err) {
		t.Fatalf("want a soft error for the malformed additional GUTI, got %v", err)
	}

	if !decoded(context.Background(), "AttachRequest", err) {
		t.Fatal("the MME must still act on the request (TS 24.301 §7.7.1)")
	}

	if req.AdditionalGUTI != nil {
		t.Errorf("the malformed element must be absent, got %+v", req.AdditionalGUTI)
	}

	if req.EPSMobileIdentity.IMSI == nil || string(*req.EPSMobileIdentity.IMSI) != "001010000000001" {
		t.Errorf("the identity must still decode, got %+v", req.EPSMobileIdentity)
	}
}
