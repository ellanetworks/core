// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func (v SessionVisitor) target() HomeTarget {
	return HomeTarget{PartnerID: v.PartnerID, PLMN: v.HomePLMN, Serving: v.ServingPLMN, RoutingIndicator: v.RoutingIndicator}
}

func (v *VSMF) udm(ctx context.Context, deps *vsmfDeps, visitor SessionVisitor, service wire.ServiceName) (wire.Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, vsmfRequestTimeout)
	defer cancel()

	return deps.home.APIRoot(ctx, visitor.target(), wire.NFTypeUDM, wire.NFTypeSMF, service)
}

func (v *VSMF) do(ctx context.Context, deps *vsmfDeps, visitor SessionVisitor, instance string, req HomeRequest) (HomeAnswer, error) {
	ctx, cancel := context.WithTimeout(ctx, vsmfRequestTimeout)
	defer cancel()

	req.Requester = wire.NFTypeSMF

	answer, err := deps.home.Do(ctx, visitor.PartnerID, req)
	if err == nil {
		return answer, nil
	}

	if pd, ok := ProblemOf(err); (!ok || vsmfTransient(pd.Status)) && instance != "" {
		deps.home.Forget(visitor.PartnerID, instance)
	}

	return answer, err
}

func vsmfHomeError(err error, problem func(wire.ProblemDetails) error) error {
	if pd, ok := ProblemOf(err); ok {
		return problem(pd)
	}

	return vsmfUnreachable(err)
}

func (v *VSMF) fetchSMData(ctx context.Context, deps *vsmfDeps, supi etsi.SUPI, visitor SessionVisitor, home models.Snssai, dnn string) (wire.DnnConfiguration, string, error) {
	c, err := v.udm(ctx, deps, visitor, wire.ServiceNudmSDM)
	if err != nil {
		return wire.DnnConfiguration{}, "", vsmfUnreachable(err)
	}

	snssai := wire.SnssaiFromModel(home)
	serving := visitor.ServingPLMN

	values, err := wire.SDMQuery{SingleNssai: &snssai, Dnn: dnn, PlmnID: &serving}.Values()
	if err != nil {
		return wire.DnnConfiguration{}, "", newSessionError(fgs.GSMCauseRequestRejectedUnspecified, err)
	}

	answer, err := v.do(ctx, deps, visitor, c.NfInstanceID, HomeRequest{
		Method: http.MethodGet,
		URL:    JoinAPIRoot(c.APIRoot, wire.SDMSmDataPath(supi.String())) + "?" + values.Encode(),
		Accept: []int{http.StatusOK},
	})
	if err != nil {
		return wire.DnnConfiguration{}, "", vsmfHomeError(err, vsmfSDMError)
	}

	data, err := wire.Decode[wire.SmSubsData](answer.Body)
	if err != nil {
		return wire.DnnConfiguration{}, "", newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: %w", ErrSessionHomeData, err))
	}

	_, cfg, err := data.Find(snssai, dnn)

	switch {
	case errors.Is(err, wire.ErrNotSubscribed):
		return wire.DnnConfiguration{}, "", newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: %s %q", ErrSessionNotSubscribed, snssai, dnn))
	case err != nil:
		return wire.DnnConfiguration{}, "", newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: %w", ErrSessionHomeData, err))
	}

	return cfg, c.NfInstanceID, nil
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

func (v *VSMF) subscribe(ctx context.Context, deps *vsmfDeps, supi etsi.SUPI, d *smData) {
	v.mu.Lock()
	ue := v.ues[supi.IMSI()]
	live := ue != nil && ue.data[smKeyOf(d.snssai, d.dnn)] == d && d.subscription == ""
	visitor, snssai, dnn := d.visitor, wire.SnssaiFromModel(d.snssai), d.dnn
	v.mu.Unlock()

	if !live {
		return
	}

	log := v.logger.With(zap.String("supi", supi.String()), zap.String("dnn", dnn))

	callbackURI := v.callbackURI(deps.n32, supi, visitor.ServingPLMN, sepp.CallbackSDMChange)
	if callbackURI == "" {
		log.Warn("No SDM subscription for the visitor's session management data: this node has no SEPP identity")
		return
	}

	c, err := v.udm(ctx, deps, visitor, wire.ServiceNudmSDM)
	if err != nil {
		log.Warn("Could not subscribe to the visitor's session management data", zap.Error(err))
		return
	}

	serving := visitor.ServingPLMN

	answer, err := v.do(ctx, deps, visitor, c.NfInstanceID, HomeRequest{
		Method: http.MethodPost,
		URL:    JoinAPIRoot(c.APIRoot, wire.SDMSubscriptionsPath(supi.String())),
		Body: wire.SdmSubscription{
			NfInstanceID:          v.instanceID(serving, deps.n32.SEPPID()),
			ImplicitUnsubscribe:   true,
			CallbackReference:     callbackURI,
			MonitoredResourceUris: []string{wire.SDMSmDataPath(supi.String())},
			SingleNssai:           &snssai,
			Dnn:                   dnn,
			PlmnID:                &serving,
		},
		Accept: []int{http.StatusCreated},
	})
	if err != nil {
		log.Warn("Could not subscribe to the visitor's session management data", zap.Error(err))
		return
	}

	location, err := vsmfLocation(answer.Location)
	if err != nil {
		log.Warn("The home UDM returned an unusable SDM subscription", zap.Error(err))
		return
	}

	var expires time.Time

	if created, err := wire.Decode[wire.SdmSubscription](answer.Body); err == nil && created.Expires != "" {
		expires, _ = created.Expires.Time()
	}

	v.mu.Lock()
	d.subscription, d.expires = location, expires
	v.scheduleRenewalLocked(supi, d)
	v.mu.Unlock()
}

func (v *VSMF) scheduleRenewalLocked(supi etsi.SUPI, d *smData) {
	if d.renewal != nil {
		d.renewal.Stop()
		d.renewal = nil
	}

	if d.expires.IsZero() {
		return
	}

	remaining := d.expires.Sub(v.now())
	delay := max(remaining-min(sdmRenewMargin, remaining/2), 0)

	d.renewal = time.AfterFunc(delay, func() {
		v.queue.Go(supi.IMSI(), func() { v.renew(supi, d) })
	})
}

func (v *VSMF) renew(supi etsi.SUPI, d *smData) {
	ctx := context.Background()
	deps := v.deps.Load()

	v.mu.Lock()
	ue := v.ues[supi.IMSI()]
	live := ue != nil && ue.data[smKeyOf(d.snssai, d.dnn)] == d
	visitor, uri := d.visitor, d.subscription
	v.mu.Unlock()

	if !live || uri == "" || deps.home == nil {
		return
	}

	proposed := v.now().Add(sdmSubscriptionLifetime).UTC()

	answer, err := v.do(ctx, deps, visitor, "", HomeRequest{
		Method:      http.MethodPatch,
		URL:         uri,
		Body:        wire.SdmSubsModification{Expires: wire.DateTimeOf(proposed)},
		ContentType: wire.ContentTypeMergePatch,
		Accept:      []int{http.StatusOK, http.StatusNoContent},
	})

	if pd, ok := ProblemOf(err); ok && pd.Status == http.StatusNotFound {
		v.mu.Lock()
		d.subscription, d.expires = "", time.Time{}
		v.mu.Unlock()

		v.subscribe(ctx, deps, supi, d)

		return
	}

	if err != nil {
		v.logger.Warn("Could not renew the SDM subscription to the visitor's session management data", zap.String("supi", supi.String()), zap.Error(err))
		return
	}

	expires := proposed

	if answer.Status == http.StatusOK {
		if res, err := wire.Decode[wire.SdmSubsModifyResult](answer.Body); err == nil && res.Subscription != nil && res.Subscription.Expires != "" {
			if t, err := res.Subscription.Expires.Time(); err == nil {
				expires = t
			}
		}
	}

	v.mu.Lock()
	if d.subscription == uri {
		d.expires = expires
		v.scheduleRenewalLocked(supi, d)
	}
	v.mu.Unlock()
}

func (v *VSMF) unsubscribe(ctx context.Context, deps *vsmfDeps, d *smData) {
	if deps.home == nil {
		return
	}

	_, err := v.do(ctx, deps, d.visitor, "", HomeRequest{Method: http.MethodDelete, URL: d.subscription, Accept: []int{http.StatusNoContent, http.StatusNotFound}})
	if err != nil {
		v.logger.Warn("Could not unsubscribe from the visitor's session management data", zap.String("dnn", d.dnn), zap.Error(err))
	}
}

func (v *VSMF) register(ctx context.Context, s *vsmfSession) error {
	deps := v.deps.Load()

	v.mu.Lock()
	done := s.closed || s.registered
	v.mu.Unlock()

	if done {
		return nil
	}

	if deps.home == nil {
		return fmt.Errorf("%w: this node has no N32 interface", ErrSessionHomeUnreachable)
	}

	c, err := v.udm(ctx, deps, s.visitor, wire.ServiceNudmUECM)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSessionHomeUnreachable, err)
	}

	psi := int(s.psi)
	snssai := wire.SnssaiFromModel(s.rules.HomeSnssai)
	serving := s.visitor.ServingPLMN

	_, err = v.do(ctx, deps, s.visitor, c.NfInstanceID, HomeRequest{
		Method: http.MethodPut,
		URL:    JoinAPIRoot(c.APIRoot, wire.UECMSmfRegistrationPath(s.supi.String(), psi)),
		Body: wire.SmfRegistration{
			SmfInstanceID:    v.instanceID(serving, deps.n32.SEPPID()),
			PduSessionID:     &psi,
			SingleNssai:      &snssai,
			Dnn:              s.dnn,
			PlmnID:           &serving,
			DeregCallbackURI: v.callbackURI(deps.n32, s.supi, serving, sepp.CallbackUECMDereg),
		},
		Accept: []int{http.StatusCreated, http.StatusOK, http.StatusNoContent},
	})
	if err != nil {
		if pd, ok := ProblemOf(err); ok {
			return vsmfUECMError(pd)
		}

		return fmt.Errorf("%w: %w", ErrSessionHomeUnreachable, err)
	}

	v.mu.Lock()
	s.registered, s.uecmRoot = true, c.APIRoot
	v.mu.Unlock()

	return nil
}

func (v *VSMF) deregister(ctx context.Context, deps *vsmfDeps, s *vsmfSession) {
	if deps.home == nil {
		return
	}

	_, err := v.do(ctx, deps, s.visitor, "", HomeRequest{
		Method: http.MethodDelete,
		URL:    JoinAPIRoot(s.uecmRoot, wire.UECMSmfRegistrationPath(s.supi.String(), int(s.psi))),
		Accept: []int{http.StatusNoContent, http.StatusNotFound},
	})
	if err != nil {
		v.logger.Warn("Could not deregister the visitor's PDU session at the home UDM", zap.String("supi", s.supi.String()), zap.Uint8("pduSessionID", s.psi), zap.Error(err))
	}
}

func (v *VSMF) refresh(ctx context.Context, supi etsi.SUPI) {
	deps := v.deps.Load()

	v.mu.Lock()

	ue := v.ues[supi.IMSI()]
	if ue == nil {
		v.mu.Unlock()
		return
	}

	entries := make([]*smData, 0, len(ue.data))
	for _, d := range ue.data {
		entries = append(entries, d)
	}
	v.mu.Unlock()

	if deps.home == nil {
		return
	}

	for _, d := range entries {
		cfg, instance, err := v.fetchSMData(ctx, deps, supi, d.visitor, d.snssai, d.dnn)

		var (
			ambr  *models.Ambr
			rules = d.rules
			se    *SessionError
		)

		if err == nil {
			ambr, rules, err = vsmfRules(cfg, d.snssai)
		}

		if err != nil && (!errors.As(err, &se) || se.Cause == fgs.GSMCauseNetworkFailure) {
			v.logger.Warn("Could not refresh the visitor's session management data", zap.String("supi", supi.String()), zap.String("dnn", d.dnn), zap.Error(err))
			continue
		}

		v.mu.Lock()

		d.subscribed = err == nil
		if err == nil {
			d.sessionAMBR, d.rules, d.instance = ambr, rules, instance
		}
		v.mu.Unlock()
	}
}
