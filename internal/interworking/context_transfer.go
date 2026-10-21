// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"errors"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

var (
	ErrUnknownUEContext       = errors.New("interworking: the peer holds no context for this identity")
	ErrIntegrityCheckFailed   = errors.New("interworking: the peer could not verify the enclosed NAS message")
	ErrNoTransferableSessions = errors.New("interworking: the peer holds no session the target system can adopt")
)

type MMContextRequest struct {
	MappedEPSGUTI eps.GUTI
	EPSNAS        []byte
}

type MMContextResponse struct {
	SUPI                etsi.SUPI
	Security            EPSSecurityContext
	UENetworkCapability eps.UENetworkCapability
	PDNConnections      []PDNConnection
	AMBRUplink          models.BitRate
	AMBRDownlink        models.BitRate
}

type EPSContextRequest struct {
	Mapped5GGUTI fgs.GUTI
	EPSNAS       []byte
}

type EPSContextResponse struct {
	SUPI           etsi.SUPI
	Security       EPSSecurityContext
	PDNConnections []PDNConnection
	AMBRUplink     models.BitRate
	AMBRDownlink   models.BitRate
}

type ArrivingSessions struct {
	PDN []PDNConnection
}
