// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func (v *VSMF) udm(ctx context.Context, target HPLMNTarget, service wire.ServiceName) (wire.Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, vsmfRequestTimeout)
	defer cancel()

	return v.home.APIRoot(ctx, target, wire.NFTypeUDM, wire.NFTypeSMF, service)
}

func (v *VSMF) do(ctx context.Context, target HPLMNTarget, instance string, req HPLMNRequest) (HPLMNAnswer, error) {
	ctx, cancel := context.WithTimeout(ctx, vsmfRequestTimeout)
	defer cancel()

	req.Requester = wire.NFTypeSMF
	req.Producer = wire.NFTypeUDM

	answer, err := v.home.Do(ctx, target, req)
	if err == nil {
		return answer, nil
	}

	if pd, ok := ProblemOf(err); (!ok || vsmfTransient(pd.Status)) && instance != "" {
		v.home.Forget(target.PartnerID, instance)
	}

	return answer, err
}

func vsmfHPLMNError(err error, problem func(wire.ProblemDetails) error) error {
	if pd, ok := ProblemOf(err); ok {
		return problem(pd)
	}

	return vsmfUnreachable(err)
}

type smAnswer struct {
	cfg      wire.DnnConfiguration
	instance string
	barred   bool
}

func (v *VSMF) fetchSMData(ctx context.Context, supi etsi.SUPI, target HPLMNTarget, home models.Snssai, dnn string) (smAnswer, error) {
	c, err := v.udm(ctx, target, wire.ServiceNudmSDM)
	if err != nil {
		return smAnswer{}, vsmfUnreachable(err)
	}

	snssai := wire.SnssaiFromModel(home)
	serving := target.Serving

	values, err := wire.SDMQuery{SingleNssai: &snssai, Dnn: dnn, PlmnID: &serving}.Values()
	if err != nil {
		return smAnswer{}, newSessionError(fgs.GSMCauseRequestRejectedUnspecified, err)
	}

	answer, err := v.do(ctx, target, c.NfInstanceID, HPLMNRequest{
		Method:  http.MethodGet,
		URL:     JoinAPIRoot(c.APIRoot, wire.SDMSmDataPath(supi.String())) + "?" + values.Encode(),
		Service: wire.ServiceNudmSDM,
		OAuth2:  c.OAuth2Required,
		Accept:  []int{http.StatusOK},
	})
	if err != nil {
		return smAnswer{}, vsmfHPLMNError(err, vsmfSDMError)
	}

	data, err := wire.Decode[wire.SmSubsData](answer.Body)
	if err != nil {
		return smAnswer{}, newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: %w", ErrSessionHPLMNData, err))
	}

	sm, cfg, err := data.Find(snssai, dnn)

	switch {
	case errors.Is(err, wire.ErrNotSubscribed):
		return smAnswer{}, newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: %s %q", ErrSessionNotSubscribed, snssai, dnn))
	case err != nil:
		return smAnswer{}, newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: %w", ErrSessionHPLMNData, err))
	}

	return smAnswer{cfg: cfg, instance: c.NfInstanceID, barred: sm.OdbPacketServices.BarsLocalBreakout()}, nil
}

func vsmfLocation(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if u.Scheme != "http" && u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("location %q is not an apiRoot-based URI", location)
	}

	if _, ok := wire.ParseNetworkFQDN(u.Hostname()); !ok {
		return "", fmt.Errorf("location %q is not in a 5GC network domain", location)
	}

	return u.String(), nil
}

func (v *VSMF) subscribe(ctx context.Context, s *vsmfSession) {
	v.mu.Lock()
	skip := s.closed || s.subscription != ""
	v.mu.Unlock()

	if skip {
		return
	}

	log := v.logger.With(zap.String("supi", s.supi.String()), zap.String("dnn", s.dnn))

	callbackURI := v.callbackURI(s.supi, s.target.Serving, sepp.CallbackSDMChange)
	if callbackURI == "" {
		log.Warn("No SDM subscription for the visitor's session management data: this node has no SEPP identity")
		return
	}

	c, err := v.udm(ctx, s.target, wire.ServiceNudmSDM)
	if err != nil {
		log.Warn("Could not subscribe to the visitor's session management data; retrying at the next reconcile", zap.Error(err))
		return
	}

	serving := s.target.Serving
	snssai := wire.SnssaiFromModel(s.snssai)

	answer, err := v.do(ctx, s.target, c.NfInstanceID, HPLMNRequest{
		Method:  http.MethodPost,
		URL:     JoinAPIRoot(c.APIRoot, wire.SDMSubscriptionsPath(s.supi.String())),
		Service: wire.ServiceNudmSDM,
		OAuth2:  c.OAuth2Required,
		Body: wire.SdmSubscription{
			NfInstanceID:          v.instanceID(serving),
			ImplicitUnsubscribe:   true,
			CallbackReference:     callbackURI,
			MonitoredResourceUris: []string{wire.SDMSmDataPath(s.supi.String())},
			SingleNssai:           &snssai,
			Dnn:                   s.dnn,
			PlmnID:                &serving,
		},
		Accept: []int{http.StatusCreated},
	})
	if err != nil {
		log.Warn("Could not subscribe to the visitor's session management data; retrying at the next reconcile", zap.Error(err))
		return
	}

	location, err := vsmfLocation(answer.Location)
	if err != nil {
		log.Warn("The home UDM returned an unusable SDM subscription", zap.Error(err))
		return
	}

	var expires time.Time

	if sub, err := wire.Decode[wire.SdmSubscription](answer.Body); err == nil && sub.Expires != "" {
		if t, err := sub.Expires.Time(); err == nil {
			expires = t
		}
	}

	v.mu.Lock()
	s.subscription, s.expires = location, expires
	v.mu.Unlock()
}

func (v *VSMF) resubscribe(ctx context.Context, s *vsmfSession) {
	v.mu.Lock()
	closed, uri, expires := s.closed, s.subscription, s.expires
	v.mu.Unlock()

	switch {
	case closed:
		return
	case uri == "":
		v.subscribe(ctx, s)
		return
	case !renewDue(expires, v.now()):
		return
	}

	proposed := v.now().Add(sdmRenewLifetime).UTC()

	answer, err := v.do(ctx, s.target, "", HPLMNRequest{
		Method:      http.MethodPatch,
		URL:         uri,
		Body:        wire.SdmSubsModification{Expires: wire.DateTimeOf(proposed)},
		ContentType: wire.ContentTypeMergePatch,
		Service:     wire.ServiceNudmSDM,
		Accept:      []int{http.StatusOK, http.StatusNoContent},
	})
	if err != nil {
		v.logger.Warn("Could not renew the SDM subscription to the visitor's session management data; subscribing again at the next reconcile",
			zap.String("supi", s.supi.String()), zap.String("dnn", s.dnn), zap.Error(err))

		v.mu.Lock()
		if s.subscription == uri {
			s.subscription, s.expires = "", time.Time{}
		}
		v.mu.Unlock()

		return
	}

	next := proposed

	if answer.Status == http.StatusOK {
		if res, err := wire.Decode[wire.SdmSubsModifyResult](answer.Body); err == nil && res.Subscription != nil && res.Subscription.Expires != "" {
			if t, err := res.Subscription.Expires.Time(); err == nil {
				next = t
			}
		}
	}

	v.mu.Lock()
	if s.subscription == uri {
		s.expires = next
	}
	v.mu.Unlock()
}

func (v *VSMF) unsubscribe(ctx context.Context, s *vsmfSession, uri string) {
	_, err := v.do(ctx, s.target, "", HPLMNRequest{Method: http.MethodDelete, URL: uri, Service: wire.ServiceNudmSDM, Accept: []int{http.StatusNoContent, http.StatusNotFound}})
	if err != nil {
		v.logger.Warn("Could not unsubscribe from the visitor's session management data", zap.String("dnn", s.dnn), zap.Error(err))
	}
}

func (v *VSMF) register(ctx context.Context, s *vsmfSession) error {
	v.mu.Lock()
	done := s.closed || s.registered
	s.tried = true
	v.mu.Unlock()

	if done {
		return nil
	}

	c, err := v.udm(ctx, s.target, wire.ServiceNudmUECM)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSessionHPLMNUnreachable, err)
	}

	psi := int(s.psi)
	snssai := wire.SnssaiFromModel(s.snssai)
	serving := s.target.Serving
	instanceID := v.instanceID(serving)

	reg := wire.SmfRegistration{
		SmfInstanceID:    instanceID,
		PduSessionID:     &psi,
		SingleNssai:      &snssai,
		Dnn:              s.dnn,
		PlmnID:           &serving,
		DeregCallbackURI: v.callbackURI(s.supi, serving, sepp.CallbackUECMDereg),
	}

	if s.iwk {
		reg.PgwFqdn = serving.EPCNodeFQDN(wire.FQDNLabelPGW + strconv.Itoa(v.n32.SEPPID()))
	}

	_, err = v.do(ctx, s.target, c.NfInstanceID, HPLMNRequest{
		Method:  http.MethodPut,
		URL:     JoinAPIRoot(c.APIRoot, wire.UECMSmfRegistrationPath(s.supi.String(), psi)),
		Body:    reg,
		Service: wire.ServiceNudmUECM,
		OAuth2:  c.OAuth2Required,
		Accept:  []int{http.StatusCreated, http.StatusOK, http.StatusNoContent},
	})
	if err != nil {
		if pd, ok := ProblemOf(err); ok {
			return vsmfUECMError(pd)
		}

		return fmt.Errorf("%w: %w", ErrSessionHPLMNUnreachable, err)
	}

	v.mu.Lock()
	s.registered, s.uecmRoot, s.instanceID = true, c.APIRoot, instanceID
	v.mu.Unlock()

	v.queue.Go(s.supi.IMSI(), func() { v.subscribe(context.WithoutCancel(ctx), s) })

	return nil
}

func (v *VSMF) deregister(ctx context.Context, s *vsmfSession) {
	query := url.Values{"smf-instance-id": []string{s.instanceID}}

	_, err := v.do(ctx, s.target, "", HPLMNRequest{
		Method:  http.MethodDelete,
		URL:     JoinAPIRoot(s.uecmRoot, wire.UECMSmfRegistrationPath(s.supi.String(), int(s.psi))) + "?" + query.Encode(),
		Service: wire.ServiceNudmUECM,
		Accept:  []int{http.StatusNoContent, http.StatusNotFound},
	})
	if err != nil {
		v.logger.Warn("Could not deregister the visitor's PDU session at the home UDM", zap.String("supi", s.supi.String()), zap.Uint8("pduSessionID", s.psi), zap.Error(err))
	}
}

func (v *VSMF) refresh(ctx context.Context, s *vsmfSession) {
	answer, err := v.fetchSMData(ctx, s.supi, s.target, s.snssai, s.dnn)

	var (
		rules smf.HPLMNRules
		se    *SessionError
	)

	if err == nil {
		rules, err = vsmfRules(answer.cfg, s.snssai)
	}

	if err != nil && (!errors.As(err, &se) || se.Cause == fgs.GSMCauseNetworkFailure) {
		v.logger.Warn("Could not refresh the visitor's session management data", zap.String("supi", s.supi.String()), zap.String("dnn", s.dnn), zap.Error(err))
		return
	}

	v.mu.Lock()
	if err == nil {
		s.rules = rules
	} else {
		s.rules.Withdrawn = true
	}
	v.mu.Unlock()
}
