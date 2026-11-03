// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

var ErrLocationPrivacy = errors.New("roaming: the home network does not allow location by an O&M client of the visited network")

const (
	LCSPrivacyLifetime = time.Hour
	LCSPrivacyRetry    = time.Minute
)

type LCSPrivacy struct {
	Data    *wire.LcsPrivacyData
	Failure string
	Expires time.Time
}

func NewLCSPrivacy(data wire.LcsPrivacyData, err error, now time.Time) LCSPrivacy {
	if err != nil {
		return LCSPrivacy{Failure: err.Error(), Expires: now.Add(LCSPrivacyRetry)}
	}

	return LCSPrivacy{Data: &data, Expires: now.Add(LCSPrivacyLifetime)}
}

func (p *LCSPrivacy) Fresh(now time.Time) bool {
	return p != nil && now.Before(p.Expires)
}

func (p LCSPrivacy) Check(now time.Time) error {
	switch {
	case p.Data == nil:
		return fmt.Errorf("%w: the home LCS privacy profile is unavailable: %s", ErrLocationPrivacy, p.Failure)
	case p.Data.Lpi != nil && p.Data.Lpi.Disallows(now):
		return fmt.Errorf("%w: the location privacy indication disallows location", ErrLocationPrivacy)
	case !p.Data.AllowsPlmnOperatorClass(wire.LcsClientOMInVPLMN):
		return fmt.Errorf("%w: the PLMN operator class does not list %s", ErrLocationPrivacy, wire.LcsClientOMInVPLMN)
	default:
		return nil
	}
}

func (h *AMFHome) LCSPrivacyData(ctx context.Context, t HomeTarget, supi string) (wire.LcsPrivacyData, error) {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeUDM, wire.NFTypeAMF, wire.ServiceNudmSDM)
	if err != nil {
		return wire.LcsPrivacyData{}, err
	}

	answer, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodGet,
		URL:       JoinAPIRoot(c.APIRoot, wire.SDMLcsPrivacyDataPath(supi)),
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusOK},
	})
	if err != nil {
		h.forgetOnTransportError(t.PartnerID, c, err)
		return wire.LcsPrivacyData{}, err
	}

	return DecodeAnswer[wire.LcsPrivacyData](answer)
}
