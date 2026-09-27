// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"testing"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	lmmodels "github.com/ellanetworks/core/internal/lmf/models"
)

func encodeProvideLocationInformationWithoutEstimate(t *testing.T) []byte {
	t.Helper()

	body := &lpptype.LPPMessageBody{
		C1: &lpptype.LPPMessageBodyC1{
			ProvideLocationInformation: &lpptype.ProvideLocationInformation{
				CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
					C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{
						ProvideLocationInformationR9: &lpptype.ProvideLocationInformationR9IEs{
							CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{
								LocationError: &lpptype.LocationError{LocationFailureCause: 1},
							},
						},
					},
				},
			},
		},
	}

	b, err := encodeLPPMessage(0x02, lpptype.InitiatorTargetDevice, body, true, 0)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestDecodeProvideLocationInformationWithoutEstimate(t *testing.T) {
	decoded, err := DecodeLPPMessage(encodeProvideLocationInformationWithoutEstimate(t))
	if err != nil {
		t.Fatal(err)
	}

	pli := decoded.ProvideLocationInformation
	if pli == nil {
		t.Fatal("ProvideLocationInformation not decoded")
	}

	if pli.HasLocationEstimate {
		t.Error("HasLocationEstimate = true, want false")
	}

	if !pli.LocationError {
		t.Error("LocationError = false, want true")
	}
}

func TestDecodeProvideLocationInformationWithEstimate(t *testing.T) {
	b, err := EncodeProvideLocationInformation(0x02, 450000000, 214500000, 10000, 10, 15)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeLPPMessage(b)
	if err != nil {
		t.Fatal(err)
	}

	if !decoded.ProvideLocationInformation.HasLocationEstimate || decoded.ProvideLocationInformation.LocationError {
		t.Errorf("ProvideLocationInformation = %+v, want an estimate and no error", decoded.ProvideLocationInformation)
	}
}

func TestSessionRejectsLocationWithoutEstimate(t *testing.T) {
	s := NewSession("imsi-001010000000001", "session-1", "agnss")
	s.state = LocationRequested

	completed := false

	s.SetTransport(nil, func(*lmmodels.LocationResult) error {
		completed = true
		return nil
	}, nil, nil, nil)

	err := s.HandleResponse(&models.ProvideLocationInformation{LocationError: true})
	if err == nil {
		t.Fatal("HandleResponse accepted a ProvideLocationInformation with no estimate")
	}

	if s.State() == LocationReceived || completed {
		t.Error("session completed without a location estimate")
	}
}
