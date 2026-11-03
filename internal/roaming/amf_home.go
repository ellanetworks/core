// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

var ErrUnsupportedAuthType = errors.New("roaming: the home AUSF selected an authentication method other than 5G AKA")

type Challenge struct {
	RAND       [16]byte
	AUTN       [16]byte
	HXRESStar  [16]byte
	ConfirmURI string
}

type SDMSubscription struct {
	URI     string
	ID      string
	Expires wire.DateTime
}

type AMFHome struct {
	sbi *HomeSBI
}

func NewAMFHome(sbi *HomeSBI) *AMFHome {
	return &AMFHome{sbi: sbi}
}

func (h *AMFHome) Authenticate(ctx context.Context, t HomeTarget, supiOrSuci string, resync *wire.ResynchronizationInfo) (Challenge, error) {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeAUSF, wire.NFTypeAMF, wire.ServiceNausfAuth)
	if err != nil {
		return Challenge{}, err
	}

	answer, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method: http.MethodPost,
		URL:    JoinAPIRoot(c.APIRoot, wire.UEAuthenticationsPath()),
		Body: wire.AuthenticationInfo{
			SupiOrSuci:            supiOrSuci,
			ServingNetworkName:    t.Serving.ServingNetworkName(),
			ResynchronizationInfo: resync,
			RoutingIndicator:      t.RoutingIndicator,
		},
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusCreated},
	})
	if err != nil {
		h.forgetOnTransportError(t.PartnerID, c, err)
		return Challenge{}, err
	}

	authCtx, err := DecodeAnswer[wire.UEAuthenticationCtx](answer)
	if err != nil {
		return Challenge{}, err
	}

	if authCtx.AuthType != wire.AuthType5GAKA {
		return Challenge{}, fmt.Errorf("%w: %s", ErrUnsupportedAuthType, authCtx.AuthType)
	}

	href, _ := authCtx.Href(wire.LinkRel5GAKA)

	rand, hxres, autn, err := authCtx.AuthData.Av5gAka.Vector()
	if err != nil {
		return Challenge{}, fmt.Errorf("%w: %w", ErrHomeAnswer, err)
	}

	return Challenge{RAND: rand, AUTN: autn, HXRESStar: hxres, ConfirmURI: href}, nil
}

func (h *AMFHome) Confirm(ctx context.Context, t HomeTarget, confirmURI, supiOrSuci string, resStar [16]byte) (string, [32]byte, error) {
	answer, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodPut,
		URL:       confirmURI,
		Body:      wire.ConfirmationWithResStar(resStar),
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusOK},
	})
	if err != nil {
		return "", [32]byte{}, err
	}

	resp, err := DecodeAnswer[wire.ConfirmationDataResponse](answer)
	if err != nil {
		return "", [32]byte{}, err
	}

	return resp.Success(supiOrSuci)
}

func (h *AMFHome) Unconfirmed(ctx context.Context, t HomeTarget, confirmURI string) error {
	_, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodPut,
		URL:       confirmURI,
		Body:      wire.ConfirmationData{},
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusOK},
	})

	return err
}

func (h *AMFHome) RemoveAuthentication(ctx context.Context, t HomeTarget, confirmURI string) error {
	_, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodDelete,
		URL:       confirmURI,
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusNoContent, http.StatusOK},
	})

	return err
}

func (h *AMFHome) Register(ctx context.Context, t HomeTarget, supi string, reg wire.Amf3GppAccessRegistration) error {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeUDM, wire.NFTypeAMF, wire.ServiceNudmUECM)
	if err != nil {
		return err
	}

	_, err = h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodPut,
		URL:       JoinAPIRoot(c.APIRoot, wire.UECMAmf3GppAccessPath(supi)),
		Body:      reg,
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusCreated, http.StatusOK, http.StatusNoContent},
	})
	h.forgetOnTransportError(t.PartnerID, c, err)

	return err
}

func (h *AMFHome) Deregister(ctx context.Context, t HomeTarget, supi string, guami wire.Guami) error {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeUDM, wire.NFTypeAMF, wire.ServiceNudmUECM)
	if err != nil {
		return err
	}

	_, err = h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:      http.MethodPatch,
		URL:         JoinAPIRoot(c.APIRoot, wire.UECMAmf3GppAccessPath(supi)),
		Body:        wire.PurgeModification(guami),
		ContentType: wire.ContentTypeMergePatch,
		Requester:   wire.NFTypeAMF,
		Accept:      []int{http.StatusNoContent, http.StatusOK},
	})
	h.forgetOnTransportError(t.PartnerID, c, err)

	return err
}

func (h *AMFHome) SubscriptionData(ctx context.Context, t HomeTarget, supi string) (wire.SubscriptionDataSets, error) {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeUDM, wire.NFTypeAMF, wire.ServiceNudmSDM)
	if err != nil {
		return wire.SubscriptionDataSets{}, err
	}

	serving := t.Serving

	values, err := wire.SDMQuery{DatasetNames: []wire.DataSetName{wire.DataSetAM, wire.DataSetSMFSel}, PlmnID: &serving}.Values()
	if err != nil {
		return wire.SubscriptionDataSets{}, err
	}

	answer, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodGet,
		URL:       JoinAPIRoot(c.APIRoot, wire.SDMDataSetsPath(supi)) + "?" + values.Encode(),
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusOK},
	})
	if err != nil {
		h.forgetOnTransportError(t.PartnerID, c, err)
		return wire.SubscriptionDataSets{}, err
	}

	return DecodeAnswer[wire.SubscriptionDataSets](answer)
}

func (h *AMFHome) Subscribe(ctx context.Context, t HomeTarget, supi string, sub wire.SdmSubscription) (SDMSubscription, error) {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeUDM, wire.NFTypeAMF, wire.ServiceNudmSDM)
	if err != nil {
		return SDMSubscription{}, err
	}

	answer, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodPost,
		URL:       JoinAPIRoot(c.APIRoot, wire.SDMSubscriptionsPath(supi)),
		Body:      sub,
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusCreated},
	})
	if err != nil {
		h.forgetOnTransportError(t.PartnerID, c, err)
		return SDMSubscription{}, err
	}

	created, err := DecodeAnswer[wire.SdmSubscription](answer)
	if err != nil {
		return SDMSubscription{}, err
	}

	out := SDMSubscription{URI: answer.Location, ID: created.SubscriptionID, Expires: created.Expires}

	if out.URI == "" && out.ID != "" {
		out.URI = JoinAPIRoot(c.APIRoot, wire.SDMSubscriptionPath(supi, out.ID))
	}

	if out.URI == "" {
		return SDMSubscription{}, fmt.Errorf("%w: the SDM subscription has neither a Location nor a subscriptionId", ErrHomeAnswer)
	}

	if out.ID == "" {
		out.ID = lastSegment(out.URI)
	}

	return out, nil
}

func (h *AMFHome) Unsubscribe(ctx context.Context, t HomeTarget, uri string) error {
	_, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodDelete,
		URL:       uri,
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusNoContent, http.StatusOK},
	})

	return err
}

func (h *AMFHome) Renew(ctx context.Context, t HomeTarget, uri string, expires wire.DateTime) (wire.DateTime, error) {
	answer, err := h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:      http.MethodPatch,
		URL:         uri,
		Body:        wire.SdmSubsModification{Expires: expires},
		ContentType: wire.ContentTypeMergePatch,
		Requester:   wire.NFTypeAMF,
		Accept:      []int{http.StatusOK, http.StatusNoContent},
	})
	if err != nil {
		return "", err
	}

	if answer.Status == http.StatusNoContent || len(answer.Body) == 0 {
		return expires, nil
	}

	result, err := DecodeAnswer[wire.SdmSubsModifyResult](answer)
	if err != nil {
		return "", err
	}

	if result.Subscription != nil && result.Subscription.Expires != "" {
		return result.Subscription.Expires, nil
	}

	return expires, nil
}

func (h *AMFHome) AcknowledgeSoR(ctx context.Context, t HomeTarget, supi string, ack wire.AcknowledgeInfo) error {
	c, err := h.sbi.APIRoot(ctx, t, wire.NFTypeUDM, wire.NFTypeAMF, wire.ServiceNudmSDM)
	if err != nil {
		return err
	}

	_, err = h.sbi.Do(ctx, t.PartnerID, HomeRequest{
		Method:    http.MethodPut,
		URL:       JoinAPIRoot(c.APIRoot, wire.SDMSorAckPath(supi)),
		Body:      ack,
		Requester: wire.NFTypeAMF,
		Accept:    []int{http.StatusNoContent, http.StatusOK},
	})

	return err
}

func (h *AMFHome) forgetOnTransportError(partnerID string, c wire.Candidate, err error) {
	if err != nil && errors.Is(err, ErrHomeUnreachable) && c.NfInstanceID != "" {
		h.sbi.Forget(partnerID, c.NfInstanceID)
	}
}

func lastSegment(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}

	p := u.Path
	for len(p) > 0 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}

	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}

	return p
}
