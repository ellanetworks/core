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
	// ErrUnknownUEContext reports a peer that holds no context for the presented
	// identity, so the UE has to be recovered by authenticating it afresh.
	ErrUnknownUEContext = errors.New("interworking: the peer holds no context for this identity")

	// ErrIntegrityCheckFailed reports a peer that holds the context but could not
	// verify the enclosed NAS message against it (TS 33.501 §8.2, §8.5.2 step 4).
	ErrIntegrityCheckFailed = errors.New("interworking: the peer could not verify the enclosed NAS message")
)

// MMContextRequest asks the MME for the UE's EPS context on an idle-mode change
// to 5GS: the Context Request of TS 23.502 §4.11.1.3.3 step 5a.
type MMContextRequest struct {
	// MappedEPSGUTI is the 4G-GUTI the presented 5G-GUTI reverse-maps to, which
	// the MME compares against its stored values (TS 23.003 §2.10.2.1.3).
	MappedEPSGUTI eps.GUTI

	// EPSNAS is the complete TRACKING AREA UPDATE REQUEST the UE enclosed in the
	// EPS NAS message container IE, integrity protected with the EPS security
	// context only the MME holds (TS 24.501 §8.2.6.16).
	EPSNAS []byte
}

// MMContextResponse is the MME's answer, returned only for a request whose
// enclosed TAU REQUEST verified.
type MMContextResponse struct {
	SUPI etsi.SUPI

	// Security carries the EPS context whose ULNASCount is the count the TAU
	// verified at — the K'AMF input (TS 33.501 Annex A.15.1).
	Security EPSSecurityContext

	UENetworkCapability eps.UENetworkCapability
	PDNConnections      []PDNConnection
	AMBRUplink          models.BitRate
	AMBRDownlink        models.BitRate
}

// EPSContextRequest asks the AMF for the UE's context on an idle-mode change to
// EPS: the Context Request of TS 23.502 §4.11.1.3.2 step 3.
type EPSContextRequest struct {
	// Mapped5GGUTI is the 5G-GUTI the presented 4G-GUTI reverse-maps to
	// (TS 23.003 §2.10.2.2.3).
	Mapped5GGUTI fgs.GUTI

	// EPSNAS is the complete TRACKING AREA UPDATE REQUEST as it arrived at the
	// MME, integrity protected with the 5G security context the AMF holds
	// (TS 33.501 §8.5.2 step 3).
	EPSNAS []byte
}

// EPSContextResponse is the AMF's answer, returned only for a request whose TAU
// REQUEST verified.
type EPSContextResponse struct {
	SUPI etsi.SUPI

	// Security is the mapped EPS context alone: TS 33.501 §8.5.2 step 5 — "The
	// AMF shall never transfer 5G security parameters to an entity outside the
	// 5G system."
	Security EPSSecurityContext

	PDNConnections []PDNConnection
	AMBRUplink     models.BitRate
	AMBRDownlink   models.BitRate
}
