// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

const T3346CapacityBackoff = 15 * time.Minute

var (
	ErrNoSubscriptionData = errors.New("the home HSS returned no subscription data")
	ErrMalformedVector    = errors.New("the home HSS returned a malformed E-UTRAN vector")
)

type HPLMNClient interface {
	AuthenticationInformation(ctx context.Context, home roaming.HPLMNPeer, imsi string, visited tgpp.PLMN, resync *s6a.Resync) (s6a.EUTRANVector, string, error)
	UpdateLocation(ctx context.Context, home roaming.HPLMNPeer, req s6a.UpdateLocationRequest) (s6a.UpdateLocation, error)
	PurgeUE(ctx context.Context, home roaming.HPLMNPeer, req s6a.PurgeUERequest) (s6a.PurgeUE, error)
	Notify(ctx context.Context, home roaming.HPLMNPeer, req s6a.NotifyRequest) error
}

type RoamingStore interface {
	ListRoamingPartners(ctx context.Context) ([]db.RoamingPartner, error)
	CountSubscribersAndVisitors(ctx context.Context) (int, error)
	SetVisitorMSISDN(ctx context.Context, imsi, msisdn string) error
}

type Rejection struct {
	Cause    eps.EMMCause
	Extended *eps.ExtendedEMMCause
	BackOff  time.Duration
	Reason   string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("%s (EMM cause %s)", r.Reason, r.Cause)
}

func eutranNotAllowed(reason string) *Rejection {
	return &Rejection{Cause: eps.EMMCauseNoSuitableCellsInTrackingArea, Extended: &eps.ExtendedEMMCause{EUTRANNotAllowed: true}, Reason: reason}
}

type Resync struct {
	AUTS []byte
	RAND []byte
}

func (ue *UeContext) Visitor() (roaming.Visitor, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor == nil {
		return roaming.Visitor{}, false
	}

	return *ue.visitor, true
}

func (ue *UeContext) IsVisitor() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.visitor != nil
}

func (ue *UeContext) setVisitor(v *roaming.Visitor) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.visitor = v
	ue.visitorMSISDN = nil
}

func (ue *UeContext) learnHSS(host string) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor != nil && host != "" {
		ue.visitor.Peer.HSS = host
	}
}

func (ue *UeContext) setHPLMNSubscription(sub *s6a.SubscriptionData) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor != nil {
		ue.visitor.Subscription = sub
		ue.visitor.Confirmed = true
	}
}

func (ue *UeContext) updateHPLMNSubscription(update func(s6a.SubscriptionData) (s6a.SubscriptionData, error)) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor == nil {
		return roaming.ErrNoHPLMNSubscription
	}

	var current s6a.SubscriptionData
	if ue.visitor.Subscription != nil {
		current = *ue.visitor.Subscription
	}

	next, err := update(current)
	if err != nil {
		return err
	}

	ue.visitor.Subscription = &next

	return nil
}

func (ue *UeContext) setHPLMNRegistered(registered bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor != nil {
		ue.visitor.Registered = registered
	}
}

func (ue *UeContext) unconfirmHPLMNLocation() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor != nil {
		ue.visitor.Confirmed = false
	}
}

func (ue *UeContext) hplmnRegistrationLocked() (roaming.Visitor, bool) {
	if ue.visitor == nil || !ue.visitor.Registered {
		return roaming.Visitor{}, false
	}

	return *ue.visitor, true
}

func (m *MME) HPLMNSubscription(imsi string) (*s6a.SubscriptionData, bool) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return nil, false
	}

	v, ok := ue.Visitor()
	if !ok || v.Subscription == nil {
		return nil, false
	}

	return v.Subscription, true
}

func (m *MME) VisitorHPLMN(imsi string) (roaming.HPLMNData, bool) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return roaming.HPLMNData{}, false
	}

	v, ok := ue.Visitor()
	if !ok || v.Subscription == nil {
		return roaming.HPLMNData{}, false
	}

	return roaming.HPLMNData{PLMN: models.PlmnID{Mcc: v.HomePLMN.MCC, Mnc: v.HomePLMN.MNC}, Subscription: v.Subscription}, true
}

func (m *MME) AdmitVisitor(ctx context.Context, ue *UeContext, serving models.PlmnID) error {
	imsi := ue.IMSI()

	if m.Roaming == nil || strings.HasPrefix(imsi, serving.Mcc+serving.Mnc) {
		ue.setVisitor(nil)
		return nil
	}

	partners, err := m.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		return fmt.Errorf("list roaming partners: %w", err)
	}

	partner, plmn, ok := roaming.PartnerOfIMSI(partners, imsi)
	if !ok {
		return &Rejection{Cause: eps.EMMCausePLMNNotAllowed, Reason: "no roaming partner for the IMSI"}
	}

	if partner.DiameterPeerID == nil {
		return eutranNotAllowed("the roaming partner has no 4G agreement")
	}

	if r := withdrawnRejection(partner); r != nil {
		return r
	}

	profile, err := m.Bearer.GetProfileByID(ctx, partner.ProfileID)
	if err != nil {
		return fmt.Errorf("get profile %s of roaming partner %s: %w", partner.ProfileID, partner.Name, err)
	}

	if !profile.Allow4G {
		return eutranNotAllowed("the roaming partner profile does not allow 4G")
	}

	if err := m.checkVisitorCapacity(ctx, imsi); err != nil {
		return err
	}

	visitor, err := roaming.NewVisitor(partner, plmn)
	if err != nil {
		return err
	}

	if held, ok := m.LookupUeByIMSI(imsi); ok && held != ue {
		if v, ok := held.Visitor(); ok && v.PartnerID == visitor.PartnerID {
			visitor.Peer.HSS = v.Peer.HSS
		}
	}

	ue.setVisitor(&visitor)

	logger.From(ctx, logger.MmeLog).Info("inbound roamer", zap.String("roaming_partner", partner.Name), zap.String("home_realm", visitor.Peer.Realm))

	return nil
}

func withdrawnRejection(partner db.RoamingPartner) *Rejection {
	if !partner.Withdrawing(time.Now()) {
		return nil
	}

	return &Rejection{Cause: eps.EMMCausePLMNNotAllowed, Reason: "the roaming agreement is being withdrawn"}
}

func (m *MME) checkVisitorCapacity(ctx context.Context, imsi string) error {
	existing, err := m.Bearer.GetSubscriberOrVisitor(ctx, imsi)

	switch {
	case err == nil && existing.RoamingPartnerID != nil:
		return nil
	case err != nil && !errors.Is(err, db.ErrNotFound):
		return fmt.Errorf("get visitor %s: %w", imsi, err)
	}

	count, err := m.Roaming.CountSubscribersAndVisitors(ctx)
	if err != nil {
		return fmt.Errorf("count subscribers and visitors: %w", err)
	}

	if count >= db.MaxNumSubscribers {
		return &Rejection{Cause: eps.EMMCauseCongestion, BackOff: T3346CapacityBackoff, Reason: "maximum number of subscribers reached"}
	}

	return nil
}

func (m *MME) AuthenticationVector(ctx context.Context, ue *UeContext, serving models.PlmnID, resync *Resync) (*udm.EPSAV, error) {
	v, visitor := ue.Visitor()
	if !visitor {
		plmn, err := EncodePLMN(serving)
		if err != nil {
			return nil, fmt.Errorf("encode serving PLMN: %w", err)
		}

		var auts, rand string
		if resync != nil {
			auts, rand = hex.EncodeToString(resync.AUTS), hex.EncodeToString(resync.RAND)
		}

		return m.Cred.GenerateEPSVector(ctx, ue.IMSI(), plmn[:], auts, rand)
	}

	if m.HPLMN == nil {
		return nil, roaming.ErrHSSUnreachable
	}

	var s6aResync *s6a.Resync
	if resync != nil {
		s6aResync = &s6a.Resync{RAND: resync.RAND, AUTS: resync.AUTS}
	}

	vec, hss, err := m.HPLMN.AuthenticationInformation(ctx, v.Peer, ue.IMSI(), tgpp.PLMN{MCC: serving.Mcc, MNC: serving.Mnc}, s6aResync)
	if err != nil {
		return nil, err
	}

	ue.learnHSS(hss)

	return epsVector(vec)
}

func epsVector(v s6a.EUTRANVector) (*udm.EPSAV, error) {
	if len(v.RAND) != 16 || len(v.AUTN) != 16 || len(v.KASME) != 32 || len(v.XRES) < 4 || len(v.XRES) > 16 {
		return nil, ErrMalformedVector
	}

	av := &udm.EPSAV{XRES: v.XRES, KASME: v.KASME}
	copy(av.RAND[:], v.RAND)
	copy(av.AUTN[:], v.AUTN)

	return av, nil
}

func (m *MME) RequestAuthenticationVector(ctx context.Context, ue *UeContext, conn *UeConn, serving models.PlmnID, resync *Resync, done func(context.Context, *udm.EPSAV, error)) {
	if !ue.IsVisitor() {
		vec, err := m.AuthenticationVector(ctx, ue, serving, resync)
		done(ctx, vec, err)

		return
	}

	var (
		vec *udm.EPSAV
		err error
	)

	m.awaitHPLMN(ctx, ue, conn, "Authentication-Information", func(ctx context.Context) {
		vec, err = m.AuthenticationVector(ctx, ue, serving, resync)
	}, func(ctx context.Context) {
		done(ctx, vec, err)
	})
}

func (m *MME) RequestUpdateLocation(ctx context.Context, ue *UeContext, conn *UeConn, serving models.PlmnID, done func(context.Context, error)) {
	if !ue.IsVisitor() {
		done(ctx, nil)
		return
	}

	var err error

	m.awaitHPLMN(ctx, ue, conn, "Update-Location", func(ctx context.Context) {
		err = m.updateLocation(ctx, ue, serving, ULRInitialAttach)
	}, func(ctx context.Context) {
		done(ctx, err)
	})
}

func (m *MME) awaitHPLMN(ctx context.Context, ue *UeContext, conn *UeConn, procedure string, job, resume func(context.Context)) {
	ctx = context.WithoutCancel(ctx)
	imsi := ue.IMSI()

	conn.Await(func() {
		m.home.Do(imsi, func() { job(ctx) })
	}, func() {
		if ue.Conn() != conn {
			logger.From(ctx, logger.MmeLog).Info("UE connection gone while waiting for the home HSS; dropping the answer",
				logger.SUPIFromIMSI(imsi), zap.String("procedure", procedure))

			return
		}

		resume(ctx)
	})
}

func (m *MME) updateLocation(ctx context.Context, ue *UeContext, serving models.PlmnID, kind ULRKind) error {
	v, ok := ue.Visitor()
	if !ok {
		return nil
	}

	if m.HPLMN == nil {
		return roaming.ErrHSSUnreachable
	}

	home := v.Peer
	flags := kind.flags()

	if kind == ULRRestoration {
		home.HSS = ""
	}

	homogeneous := s6a.IMSVoiceOverPSNotSupported

	ue.setHPLMNRegistered(true)

	ula, err := m.HPLMN.UpdateLocation(ctx, home, s6a.UpdateLocationRequest{
		IMSI:                      ue.IMSI(),
		Features:                  roaming.ULRFeatures,
		Terminal:                  terminalInformation(ue),
		RATType:                   tgpp.RATEUTRAN,
		Flags:                     flags,
		VisitedPLMN:               tgpp.PLMN{MCC: serving.Mcc, MNC: serving.Mnc},
		HomogeneousIMSVoiceOverPS: &homogeneous,
	})

	var refused *s6a.ResultError
	if errors.As(err, &refused) {
		ue.setHPLMNRegistered(false)
	}

	if err != nil {
		return err
	}

	ue.learnHSS(ula.HSS.OriginHost)

	if ula.Subscription == nil {
		return ErrNoSubscriptionData
	}

	if kind != ULRRestoration {
		if err := subscriptionRestriction(*ula.Subscription); err != nil {
			return err
		}
	}

	if kind.carriesPDNConnections() && ula.Subscription.BarsVisitedPLMNAccessPoints() {
		return &Rejection{Cause: eps.EMMCauseNoEPSBearerContextActivated, Reason: "operator determined barring bars every PDN connection of the visitor"}
	}

	ue.setHPLMNSubscription(ula.Subscription)

	return nil
}

func (m *MME) recordVisitorMSISDN(ctx context.Context, ue *UeContext) {
	if m.Roaming == nil {
		return
	}

	ue.mu.Lock()

	if ue.visitor == nil || ue.visitor.Subscription == nil {
		ue.mu.Unlock()
		return
	}

	msisdn := ue.visitor.Subscription.MSISDN
	unchanged := ue.visitorMSISDN != nil && *ue.visitorMSISDN == msisdn

	ue.mu.Unlock()

	if unchanged {
		return
	}

	if err := m.Roaming.SetVisitorMSISDN(ctx, ue.IMSI(), msisdn); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to store the visitor's MSISDN", logger.SUPI(ue.Supi().String()), zap.Error(err))
		return
	}

	ue.mu.Lock()
	ue.visitorMSISDN = &msisdn
	ue.mu.Unlock()
}

func terminalInformation(ue *UeContext) *s6a.TerminalInformation {
	ue.mu.Lock()
	imei := ue.Imei
	ue.mu.Unlock()

	if imei.TACAndSNR() == "" {
		return nil
	}

	return &s6a.TerminalInformation{IMEI: imei.TACAndSNR(), SoftwareVersion: imei.SoftwareVersion()}
}

func subscriptionRestriction(sub s6a.SubscriptionData) error {
	switch {
	case sub.RoamingRestrictedDueToUnsupportedFeature:
		return &Rejection{Cause: eps.EMMCauseEPSServicesNotAllowedInThisPLMN, Reason: "roaming restricted due to an unsupported feature"}
	case sub.AccessRestrictionData != nil && *sub.AccessRestrictionData&(s6a.ARDWBEUTRANNotAllowed|s6a.ARDWBEUTRANExceptLTEMNotAllowed) != 0:
		return eutranNotAllowed("E-UTRAN not allowed by the home subscription")
	}

	return nil
}

func HPLMNRejection(err error) *Rejection {
	var r *Rejection
	if errors.As(err, &r) {
		return r
	}

	var re *s6a.ResultError
	if !errors.As(err, &re) {
		if errors.Is(err, roaming.ErrHSSUnreachable) {
			return &Rejection{Cause: eps.EMMCauseProtocolErrorUnspecified, Reason: "the home HSS cannot be reached"}
		}

		return &Rejection{Cause: eps.EMMCauseNetworkFailure, Reason: "the home HSS answered unexpectedly"}
	}

	reason := re.String()

	switch {
	case re.IsExperimental(tgpp.ResultErrorUserUnknown):
		return &Rejection{Cause: eps.EMMCauseEPSAndNonEPSServicesNotAllowed, Reason: reason}
	case re.IsExperimental(tgpp.ResultErrorRoamingNotAllowed):
		return &Rejection{Cause: roamingNotAllowedCause(re.ErrorDiagnostic), Reason: reason}
	case re.IsExperimental(tgpp.ResultErrorUnknownEPSSubscription):
		if re.ErrorDiagnostic != nil && *re.ErrorDiagnostic == s6a.ErrorDiagnosticNoGPRSDataSubscribed {
			return &Rejection{Cause: eps.EMMCauseEPSServicesNotAllowed, Reason: reason}
		}

		return &Rejection{Cause: eps.EMMCauseNoSuitableCellsInTrackingArea, Reason: reason}
	case re.IsExperimental(tgpp.ResultErrorRATNotAllowed):
		return &Rejection{Cause: eps.EMMCauseNoSuitableCellsInTrackingArea, Reason: reason}
	case !re.Experimental && (re.Code == diameter.ResultUnableToDeliver || re.Code == diameter.ResultRealmNotServed || re.Code == diameter.ResultAuthorizationRejected):
		return &Rejection{Cause: eps.EMMCauseNoSuitableCellsInTrackingArea, Reason: reason}
	default:
		return &Rejection{Cause: eps.EMMCauseNetworkFailure, Reason: reason}
	}
}

func roamingNotAllowedCause(d *s6a.ErrorDiagnostic) eps.EMMCause {
	switch {
	case d == nil:
		return eps.EMMCausePLMNNotAllowed
	case *d == s6a.ErrorDiagnosticODBHPLMNAPN, *d == s6a.ErrorDiagnosticODBVPLMNAPN:
		return eps.EMMCauseEPSServicesNotAllowedInThisPLMN
	case *d == s6a.ErrorDiagnosticODBAllAPN:
		return eps.EMMCauseNoSuitableCellsInTrackingArea
	default:
		return eps.EMMCausePLMNNotAllowed
	}
}
