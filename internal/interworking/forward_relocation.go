// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

type PDNConnection struct {
	PDUSessionID      uint8
	EPSBearerIdentity uint8
	APN               string
	Snssai            models.Snssai
}

type ENBIdentity struct {
	PlmnID         models.PlmnID
	ID             uint32
	Bits           uint8
	SelectedEPSTAI EPSTAI
}

// EPSTAI is an E-UTRAN Tracking Area Identity: a PLMN and a TAC (TS 23.003).
type EPSTAI struct {
	PlmnID models.PlmnID
	TAC    uint16
}

type FiveGSTAI struct {
	PlmnID models.PlmnID
	TAC    uint32
}

type NGRANNodeKind uint8

const (
	NGRANNodeGNB NGRANNodeKind = iota
	NGRANNodeNgENB
)

type NGRANIdentity struct {
	Kind        NGRANNodeKind
	PlmnID      models.PlmnID
	ID          uint32
	Bits        uint8
	SelectedTAI FiveGSTAI
}

type RelocationID uint64

type ForwardRelocationRequest struct {
	ID              RelocationID
	SUPI            etsi.SUPI
	SecurityContext EPSSecurityContext
	PDNConnections  []PDNConnection
	Target          ENBIdentity
	SourceToTarget  []byte
	Cause           s1ap.Cause
	UEAMBRUplink    models.BitRate
	UEAMBRDownlink  models.BitRate
}

type ForwardRelocationResponse struct {
	TargetToSource      []byte
	AcceptedPDUSessions []uint8
}

type FiveGSRelocationRequest struct {
	ID              RelocationID
	SUPI            etsi.SUPI
	SecurityContext EPSSecurityContext
	PDNConnections  []PDNConnection
	Target          NGRANIdentity
	SourceToTarget  []byte
	Cause           s1ap.Cause
	UEAMBRUplink    models.BitRate
	UEAMBRDownlink  models.BitRate
}

type FiveGSRelocationResponse struct {
	TargetToSource     []byte
	AcceptedEPSBearers []uint8
}

var (
	ErrUnknownTarget     = errors.New("interworking: the target is not connected")
	ErrTargetRefused     = errors.New("interworking: the target refused the handover")
	ErrRelocationTooLate = errors.New("interworking: the UE has already reached the target, too late to cancel")
)

type EPSPeer interface {
	ForwardRelocation(ctx context.Context, req ForwardRelocationRequest) (ForwardRelocationResponse, error)
	RelocationCancel(ctx context.Context, supi etsi.SUPI, id RelocationID) error
	RelocationComplete(ctx context.Context, supi etsi.SUPI, id RelocationID) error
}

type FiveGSPeer interface {
	ForwardRelocation(ctx context.Context, req FiveGSRelocationRequest) (FiveGSRelocationResponse, error)
	RelocationCancel(ctx context.Context, supi etsi.SUPI, id RelocationID) error
	RelocationComplete(ctx context.Context, supi etsi.SUPI, id RelocationID) error
}
