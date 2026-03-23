// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextReleaseComplete_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	msg := &ngapType.UEContextReleaseComplete{}

	assertNoPanic(t, "HandleUEContextReleaseComplete(empty IEs)", func() {
		ngap.HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandleUEContextReleaseComplete_SmContextNotFound verifies that a
// UEContextReleaseComplete referencing a PDU session ID that has no SmContext
// does NOT panic. Reproduces a nil pointer dereference on smContext.Ref when
// SmContextFindByPDUSessionID returns (nil, false) and the handler is missing
// a continue statement.
func TestHandleUEContextReleaseComplete_SmContextNotFound(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	// Create a UE in Registered state with an empty SmContextList.
	amfUe := amf.NewAmfUe()
	amfUe.ForceState(amf.Registered)
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 100,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	// Register the radio with the AMF so FindRanUeByAmfUeNgapID can find it.
	amfInstance.Radios = map[*sctp.SCTPConn]*amf.Radio{new(sctp.SCTPConn): ran}

	msg := &ngapType.UEContextReleaseComplete{}

	// AMFUENGAPID — mandatory, must match the ranUe.
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextReleaseCompleteIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextReleaseCompleteIEsValue{
			Present:     ngapType.UEContextReleaseCompleteIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 100},
		},
	})

	// RANUENGAPID — mandatory.
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextReleaseCompleteIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextReleaseCompleteIEsValue{
			Present:     ngapType.UEContextReleaseCompleteIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	// PDUSessionResourceListCxtRelCpl with a session ID that does NOT exist
	// in the UE's SmContextList. This triggers SmContextFindByPDUSessionID
	// returning (nil, false).
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextReleaseCompleteIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextReleaseCompleteIEsValue{
			Present: ngapType.UEContextReleaseCompleteIEsPresentPDUSessionResourceListCxtRelCpl,
			PDUSessionResourceListCxtRelCpl: &ngapType.PDUSessionResourceListCxtRelCpl{
				List: []ngapType.PDUSessionResourceItemCxtRelCpl{
					{
						PDUSessionID: ngapType.PDUSessionID{Value: 5}, // no SmContext for session 5
					},
				},
			},
		},
	})

	assertNoPanic(t, "HandleUEContextReleaseComplete(SmContext not found)", func() {
		ngap.HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)
	})
}
