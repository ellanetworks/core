// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

type PDNConnection struct {
	PDUSessionID      uint8
	EPSBearerIdentity uint8
	APN               string
	Snssai            models.Snssai
}

type ENBIdentity struct {
	PlmnID models.PlmnID
	ID     uint32
	Bits   uint8
	EPSTAC uint16
}

type ForwardRelocationRequest struct {
	SUPI            etsi.SUPI
	SecurityContext EPSSecurityContext
	PDNConnections  []PDNConnection
	Target          ENBIdentity
	SourceToTarget  []byte
	UEAMBRUplink    models.BitRate
	UEAMBRDownlink  models.BitRate
}

type ForwardRelocationResponse struct {
	TargetToSource      []byte
	AcceptedPDUSessions []uint8
}

var (
	ErrUnknownTarget = errors.New("interworking: the target is not connected")
	ErrTargetRefused = errors.New("interworking: the target refused the handover")
)

type EPSPeer interface {
	ForwardRelocation(ctx context.Context, req ForwardRelocationRequest) (ForwardRelocationResponse, error)
	RelocationCancel(ctx context.Context, supi etsi.SUPI) error
}

type FiveGSPeer interface {
	RelocationComplete(ctx context.Context, supi etsi.SUPI) error
}
