package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

type NGResetOpts struct {
	Cause    *ngapType.Cause
	ResetAll bool
}

func BuildNGReset(opts *NGResetOpts) (ngapType.NGAPPDU, error) {
	if opts == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("NGResetOpts is nil")
	}

	if opts.Cause == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("cause is required to build NGReset")
	}

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeNGReset
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentNGReset
	initiatingMessage.Value.NGReset = new(ngapType.NGReset)

	ngReset := initiatingMessage.Value.NGReset
	ngResetIEs := &ngReset.ProtocolIEs

	ie := ngapType.NGResetIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGResetIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)

	cause := ie.Value.Cause

	switch opts.Cause.Present {
	case ngapType.CausePresentMisc:
		cause.Present = ngapType.CausePresentMisc
		cause.Misc = new(ngapType.CauseMisc)
		cause.Misc.Value = opts.Cause.Misc.Value
	default:
		return ngapType.NGAPPDU{}, fmt.Errorf("unsupported Cause Present value: %d", opts.Cause.Present)
	}

	ngResetIEs.List = append(ngResetIEs.List, ie)

	if opts.ResetAll {
		ie = ngapType.NGResetIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDResetType
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.NGResetIEsPresentResetType
		ie.Value.ResetType = new(ngapType.ResetType)
		ie.Value.ResetType.Present = ngapType.ResetTypePresentNGInterface
		ie.Value.ResetType.NGInterface = new(ngapType.ResetAll)
		ie.Value.ResetType.NGInterface.Value = ngapType.ResetAllPresentResetAll

		ngResetIEs.List = append(ngResetIEs.List, ie)
	}

	return pdu, nil
}
