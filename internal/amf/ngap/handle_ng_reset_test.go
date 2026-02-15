// Copyright 2025 Ella Networks

package ngap_test

import (
	"context"
	"fmt"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

type ResetType int

const (
	ResetTypePresentNGInterface ResetType = iota
	ResetTypePresentPartOfNGInterface
)

type NGInterface struct {
	RanUENgapID int64
	AmfUENgapID int64
}

type NGResetOpts struct {
	ResetType         ResetType
	PartOfNGInterface []NGInterface
}

func buildNGReset(opts *NGResetOpts) (*ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{
		Present: ngapType.NGAPPDUPresentInitiatingMessage,
		InitiatingMessage: &ngapType.InitiatingMessage{
			ProcedureCode: ngapType.ProcedureCode{
				Value: ngapType.ProcedureCodeNGReset,
			},
			Criticality: ngapType.Criticality{
				Value: ngapType.CriticalityPresentReject,
			},
			Value: ngapType.InitiatingMessageValue{
				Present: ngapType.InitiatingMessagePresentNGReset,
				NGReset: &ngapType.NGReset{},
			},
		},
	}

	nGResetIEs := &pdu.InitiatingMessage.Value.NGReset.ProtocolIEs

	ie := ngapType.NGResetIEs{
		Id: ngapType.ProtocolIEID{
			Value: ngapType.ProtocolIEIDResetType,
		},
		Criticality: ngapType.Criticality{
			Value: ngapType.CriticalityPresentReject,
		},
		Value: ngapType.NGResetIEsValue{
			Present: ngapType.NGResetIEsPresentResetType,
		},
	}

	switch opts.ResetType {
	case ResetTypePresentNGInterface:
		ie.Value.ResetType = &ngapType.ResetType{
			Present: ngapType.ResetTypePresentNGInterface,
			NGInterface: &ngapType.ResetAll{
				Value: ngapType.ResetAllPresentResetAll,
			},
		}
	case ResetTypePresentPartOfNGInterface:
		ie.Value.ResetType = &ngapType.ResetType{
			Present:           ngapType.ResetTypePresentPartOfNGInterface,
			PartOfNGInterface: &ngapType.UEAssociatedLogicalNGConnectionList{},
		}
		for _, ngInterface := range opts.PartOfNGInterface {
			ueAssociatedLogicalNGConnectionItem := ngapType.UEAssociatedLogicalNGConnectionItem{}
			ueAssociatedLogicalNGConnectionItem.RANUENGAPID = &ngapType.RANUENGAPID{
				Value: ngInterface.RanUENgapID,
			}
			ueAssociatedLogicalNGConnectionItem.AMFUENGAPID = &ngapType.AMFUENGAPID{
				Value: ngInterface.AmfUENgapID,
			}
			ie.Value.ResetType.PartOfNGInterface.List = append(ie.Value.ResetType.PartOfNGInterface.List, ueAssociatedLogicalNGConnectionItem)
		}
	default:
		return nil, fmt.Errorf("unsupported ResetType: %v", opts.ResetType)
	}

	nGResetIEs.List = append(nGResetIEs.List, ie)

	ie = ngapType.NGResetIEs{
		Id: ngapType.ProtocolIEID{
			Value: ngapType.ProtocolIEIDCause,
		},
		Criticality: ngapType.Criticality{
			Value: ngapType.CriticalityPresentIgnore,
		},
		Value: ngapType.NGResetIEsValue{
			Present: ngapType.NGResetIEsPresentCause,
			Cause: &ngapType.Cause{
				Present: ngapType.CausePresentMisc,
				Misc: &ngapType.CauseMisc{
					Value: ngapType.CauseMiscPresentHardwareFailure,
				},
			},
		},
	}

	nGResetIEs.List = append(nGResetIEs.List, ie)

	return &pdu, nil
}

func TestHandleNGReset_ResetNGInterface(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs: map[int64]*amfContext.RanUe{
			0: {RanUeNgapID: 0, AmfUeNgapID: 0, Radio: &amfContext.Radio{}},
			1: {RanUeNgapID: 1, AmfUeNgapID: 1, Radio: &amfContext.Radio{}},
		},
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	ran.RanUEs[0].Radio = ran
	ran.RanUEs[1].Radio = ran

	msg, err := buildNGReset(&NGResetOpts{
		ResetType: ResetTypePresentNGInterface,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	ngap.HandleNGReset(context.Background(), ran, msg.InitiatingMessage.Value.NGReset)

	if len(fakeNGAPSender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface != nil {
		t.Fatalf("expected PartOfNGInterface to be nil, but got %v", fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface)
	}

	if len(ran.RanUEs) != 0 {
		t.Fatalf("expected all UEs to be removed from the RAN, but got %d", len(ran.RanUEs))
	}
}

func TestHandleNGReset_PartOfNGInterface(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs: map[int64]*amfContext.RanUe{
			0: {RanUeNgapID: 0, AmfUeNgapID: 0, Radio: &amfContext.Radio{}},
			1: {RanUeNgapID: 1, AmfUeNgapID: 1, Radio: &amfContext.Radio{}},
		},
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	ran.RanUEs[0].Radio = ran
	ran.RanUEs[1].Radio = ran

	msg, err := buildNGReset(&NGResetOpts{
		ResetType: ResetTypePresentPartOfNGInterface,
		PartOfNGInterface: []NGInterface{
			{RanUENgapID: 0, AmfUENgapID: 0},
		},
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	ngap.HandleNGReset(context.Background(), ran, msg.InitiatingMessage.Value.NGReset)

	if len(fakeNGAPSender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface == nil {
		t.Fatalf("expected PartOfNGInterface to be not nil")
	}

	if len(fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List) != 1 {
		t.Fatalf("expected 1 UE in PartOfNGInterface, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List[0].RANUENGAPID.Value != 0 {
		t.Fatalf("expected RANUENGAPID to be 0, but got %d", fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List[0].RANUENGAPID.Value)
	}

	if len(ran.RanUEs) != 1 {
		t.Fatalf("expected 1 UE to remain in the RAN, but got %d", len(ran.RanUEs))
	}
}
