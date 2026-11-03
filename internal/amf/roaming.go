// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const (
	T3346CapacityBackoff = 15 * time.Minute
	N32Wait              = 3 * time.Second
)

var (
	ErrNoHomeNetwork    = errors.New("amf: this node cannot reach home networks")
	ErrNotVisitor       = errors.New("amf: no visitor context for the SUPI")
	ErrHomeSUPIMismatch = errors.New("amf: the home network authenticated a SUPI outside the partner's PLMN")
	ErrReauthenticate   = errors.New("amf: the home network requires re-authentication")
)

type HomeNetwork interface {
	Authenticate(ctx context.Context, t roaming.HomeTarget, supiOrSuci string, resync *wire.ResynchronizationInfo) (roaming.Challenge, error)
	Confirm(ctx context.Context, t roaming.HomeTarget, confirmURI, supiOrSuci string, resStar [16]byte) (string, [32]byte, error)
	Unconfirmed(ctx context.Context, t roaming.HomeTarget, confirmURI string) error
	RemoveAuthentication(ctx context.Context, t roaming.HomeTarget, confirmURI string) error
	Register(ctx context.Context, t roaming.HomeTarget, supi string, reg wire.Amf3GppAccessRegistration) error
	Deregister(ctx context.Context, t roaming.HomeTarget, supi string, guami wire.Guami) error
	SubscriptionData(ctx context.Context, t roaming.HomeTarget, supi string) (wire.SubscriptionDataSets, error)
	Subscribe(ctx context.Context, t roaming.HomeTarget, supi string, sub wire.SdmSubscription) (roaming.SDMSubscription, error)
	Unsubscribe(ctx context.Context, t roaming.HomeTarget, uri string) error
	AcknowledgeSoR(ctx context.Context, t roaming.HomeTarget, supi string, ack wire.AcknowledgeInfo) error
}

type N32Gate interface {
	WaitEstablished(ctx context.Context, partnerID string, timeout time.Duration) error
}

type RoamingStore interface {
	ListRoamingPartners(ctx context.Context) ([]db.RoamingPartner, error)
	CountSubscribersAndVisitors(ctx context.Context) (int, error)
}

type Rejection struct {
	Cause      fgs.GMMCause
	BackOff    time.Duration
	AuthReject bool
	Reason     string
}

func (r *Rejection) Error() string {
	if r.AuthReject {
		return r.Reason + " (authentication reject)"
	}

	return fmt.Sprintf("%s (5GMM cause %s)", r.Reason, r.Cause)
}

func reject(cause fgs.GMMCause, reason string) *Rejection {
	return &Rejection{Cause: cause, Reason: reason}
}

func (ue *UeContext) Visitor() (roaming.Visitor5G, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor == nil {
		return roaming.Visitor5G{}, false
	}

	return *ue.visitor, true
}

func (ue *UeContext) IsVisitor() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.visitor != nil
}

func (ue *UeContext) setVisitor(v *roaming.Visitor5G) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.visitor = v
}

func (ue *UeContext) updateVisitor(f func(v *roaming.Visitor5G)) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.visitor != nil {
		f(ue.visitor)
	}
}

func (amf *AMF) VisitorHome(imsi string) (roaming.Visitor5G, bool) {
	ue, ok := amf.lookupUeByIMSI(imsi)
	if !ok {
		return roaming.Visitor5G{}, false
	}

	return ue.Visitor()
}

func homeOfUE(ue *UeContext) (string, string, bool) {
	if suci := ue.Suci(); suci != "" {
		s, err := wire.ParseSuci(suci)
		if err != nil {
			return "", "", false
		}

		return s.Mcc + s.Mnc, s.RoutingIndicator, true
	}

	if supi := ue.Supi(); supi.IsIMSI() {
		return supi.IMSI(), "", true
	}

	return "", "", false
}

func (amf *AMF) AdmitVisitor(ctx context.Context, ue *UeContext) error {
	if amf.Roaming == nil {
		return nil
	}

	prefix, routingIndicator, ok := homeOfUE(ue)
	if !ok {
		ue.setVisitor(nil)
		return nil
	}

	info, err := amf.OperatorInfo(ctx)
	if err != nil {
		return err
	}

	op := *info.Guami.PlmnID
	if strings.HasPrefix(prefix, op.Mcc+op.Mnc) {
		ue.setVisitor(nil)
		return nil
	}

	partners, err := amf.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		return fmt.Errorf("list roaming partners: %w", err)
	}

	partner, plmn, ok := roaming.PartnerOfIMSI(partners, prefix)
	if !ok {
		return reject(fgs.GMMCausePLMNNotAllowed, "no roaming partner for the home network")
	}

	if r := admissionRejection(partner, prefix); r != nil {
		return r
	}

	profile, err := amf.DBInstance.GetProfileByID(ctx, partner.ProfileID)
	if err != nil {
		return fmt.Errorf("get profile %s of roaming partner %s: %w", partner.ProfileID, partner.Name, err)
	}

	if !profile.Allow5G {
		return reject(fgs.GMMCauseN1ModeNotAllowed, "the roaming partner profile does not allow 5G")
	}

	if supi := ue.Supi(); supi.IsIMSI() {
		if err := amf.checkVisitorCapacity(ctx, supi.IMSI()); err != nil {
			return err
		}
	}

	visitor := roaming.NewVisitor5G(partner, plmn, op, routingIndicator)

	if current, ok := ue.Visitor(); ok && current.PartnerID == visitor.PartnerID && current.Home.PLMN == visitor.Home.PLMN {
		if visitor.Home.RoutingIndicator == "" {
			visitor.Home.RoutingIndicator = current.Home.RoutingIndicator
		}

		current.Home = visitor.Home
		current.ConfirmURI = ""
		visitor = current
	}

	ue.setVisitor(&visitor)

	logger.From(ctx, logger.AmfLog).Info("inbound roamer", zap.String("roaming_partner", partner.Name), zap.String("home_plmn", visitor.Home.PLMN.String()))

	return nil
}

func admissionRejection(partner db.RoamingPartner, imsi string) *Rejection {
	if partner.SEPPFQDN == nil {
		return reject(fgs.GMMCauseN1ModeNotAllowed, "the roaming partner has no 5G agreement")
	}

	w, ok := partner.Withdrawal(imsi, db.UERegistrationTypeAMF3GPPAccess, time.Now())
	if !ok {
		return nil
	}

	if w.RoamerOnly() {
		return reject(fgs.GMMCauseCongestion, "the operator is deregistering the visitor")
	}

	if w.AgreementOnly() {
		return reject(fgs.GMMCauseN1ModeNotAllowed, "the 5G agreement of the roaming partner is being withdrawn")
	}

	return reject(fgs.GMMCausePLMNNotAllowed, "the roaming agreement is being withdrawn")
}

func (amf *AMF) checkVisitorCapacity(ctx context.Context, imsi string) error {
	existing, err := amf.DBInstance.GetSubscriberOrVisitor(ctx, imsi)

	switch {
	case err == nil && existing.RoamingPartnerID != nil:
		return nil
	case err == nil:
		return reject(fgs.GMMCausePLMNNotAllowed, "the IMSI is a home subscriber of another PLMN")
	case !errors.Is(err, db.ErrNotFound):
		return fmt.Errorf("get visitor %s: %w", imsi, err)
	}

	count, err := amf.Roaming.CountSubscribersAndVisitors(ctx)
	if err != nil {
		return fmt.Errorf("count subscribers and visitors: %w", err)
	}

	if count >= db.MaxNumSubscribers {
		return &Rejection{Cause: fgs.GMMCauseCongestion, BackOff: T3346CapacityBackoff, Reason: "maximum number of subscribers reached"}
	}

	return nil
}

func (amf *AMF) awaitHome(ctx context.Context, ue *UeContext, conn *UeConn, key, procedure string, job, resume func(context.Context)) {
	ctx = context.WithoutCancel(ctx)

	conn.Await(func() {
		amf.home.Do(key, func() { job(ctx) })
	}, func() {
		if ue.Conn() != conn {
			logger.From(ctx, logger.AmfLog).Info("UE connection gone while waiting for the home network; dropping the answer",
				zap.String("procedure", procedure))

			return
		}

		resume(ctx)
	})
}

func supiOrSuci(ue *UeContext) string {
	if suci := ue.Suci(); suci != "" {
		return suci
	}

	return ue.Supi().String()
}

func authKey(ue *UeContext) string {
	if supi := ue.Supi(); supi.IsIMSI() {
		return supi.IMSI()
	}

	return ue.Suci()
}

func (amf *AMF) waitN32(ctx context.Context, v roaming.Visitor5G) error {
	if amf.N32 == nil || amf.HomeNetwork == nil {
		return fmt.Errorf("%w: %w", roaming.ErrHomeUnreachable, ErrNoHomeNetwork)
	}

	if err := amf.N32.WaitEstablished(ctx, v.PartnerID, N32Wait); err != nil {
		return fmt.Errorf("%w: %w", roaming.ErrHomeUnreachable, err)
	}

	return nil
}

func (amf *AMF) RequestAuthentication(ctx context.Context, ue *UeContext, conn *UeConn, resync *ausf.ResyncInfo, done func(context.Context, *ausf.AuthResult, error)) {
	v, visitor := ue.Visitor()
	if !visitor {
		if ue.Tai.PlmnID == nil {
			done(ctx, nil, fmt.Errorf("tai is not available in UE context"))
			return
		}

		result, err := amf.Ausf.Authenticate(ctx, ue.Suci(), *ue.Tai.PlmnID, resync)
		done(ctx, result, err)

		return
	}

	var resyncInfo *wire.ResynchronizationInfo

	if resync != nil {
		r, err := homeResync(conn, resync)
		if err != nil {
			done(ctx, nil, err)
			return
		}

		resyncInfo = r
	}

	id := supiOrSuci(ue)

	var (
		result *ausf.AuthResult
		err    error
	)

	amf.awaitHome(ctx, ue, conn, authKey(ue), "Nausf_UEAuthentication_Authenticate", func(ctx context.Context) {
		if err = amf.waitN32(ctx, v); err != nil {
			return
		}

		if v.ConfirmURI != "" {
			amf.unconfirm(ctx, v)
		}

		var c roaming.Challenge

		c, err = amf.HomeNetwork.Authenticate(ctx, v.Home, id, resyncInfo)
		if err != nil {
			return
		}

		ue.updateVisitor(func(cur *roaming.Visitor5G) {
			cur.ConfirmURI = c.ConfirmURI
			cur.Confirmed = false
		})

		result = &ausf.AuthResult{Rand: hex.EncodeToString(c.RAND[:]), Autn: hex.EncodeToString(c.AUTN[:]), HxresStar: hex.EncodeToString(c.HXRESStar[:])}
	}, func(ctx context.Context) {
		done(ctx, result, err)
	})
}

func homeResync(conn *UeConn, resync *ausf.ResyncInfo) (*wire.ResynchronizationInfo, error) {
	if conn.AuthenticationCtx == nil {
		return nil, errors.New("amf: re-synchronisation without a previous challenge")
	}

	rand, err := hex.DecodeString(conn.AuthenticationCtx.Rand)
	if err != nil || len(rand) != 16 {
		return nil, errors.New("amf: re-synchronisation with a malformed RAND")
	}

	auts, err := hex.DecodeString(resync.Auts)
	if err != nil || len(auts) != 14 {
		return nil, errors.New("amf: re-synchronisation with a malformed AUTS")
	}

	return wire.NewResynchronizationInfo([16]byte(rand), [14]byte(auts)), nil
}

func (amf *AMF) RequestConfirmation(ctx context.Context, ue *UeContext, conn *UeConn, resStar [16]byte, done func(context.Context, etsi.SUPI, []byte, error)) {
	v, visitor := ue.Visitor()
	if !visitor {
		supi, kseaf, err := amf.Ausf.Confirm(ctx, hex.EncodeToString(resStar[:]), ue.Suci())
		done(ctx, supi, kseaf, err)

		return
	}

	if v.ConfirmURI == "" {
		done(ctx, etsi.InvalidSUPI, nil, errors.New("amf: no pending home authentication"))
		return
	}

	id := supiOrSuci(ue)

	var (
		supi  etsi.SUPI
		kseaf []byte
		err   error
	)

	amf.awaitHome(ctx, ue, conn, authKey(ue), "Nausf_UEAuthentication_Authenticate (confirmation)", func(ctx context.Context) {
		var (
			s string
			k [32]byte
		)

		s, k, err = amf.HomeNetwork.Confirm(ctx, v.Home, v.ConfirmURI, id, resStar)
		if err != nil {
			return
		}

		imsi, ok := wire.IMSIFromSupi(s)
		if !ok || !strings.HasPrefix(imsi, v.Home.PLMN.Mcc+v.Home.PLMN.Mnc) {
			err = fmt.Errorf("%w: %s", ErrHomeSUPIMismatch, s)
			return
		}

		supi, err = etsi.NewSUPIFromIMSI(imsi)
		if err != nil {
			return
		}

		kseaf = k[:]

		ue.updateVisitor(func(cur *roaming.Visitor5G) {
			if cur.ConfirmURI == v.ConfirmURI {
				cur.Confirmed = true
			}
		})
	}, func(ctx context.Context) {
		done(ctx, supi, kseaf, err)
	})
}

func (amf *AMF) AuthenticationFailed(ctx context.Context, ue *UeContext) {
	v, ok := ue.Visitor()
	if !ok || v.ConfirmURI == "" || amf.HomeNetwork == nil {
		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.ConfirmURI = "" })

	ctx = context.WithoutCancel(ctx)

	amf.home.Go(authKey(ue), func() { amf.unconfirm(ctx, v) })
}

func (amf *AMF) unconfirm(ctx context.Context, v roaming.Visitor5G) {
	if err := amf.HomeNetwork.Unconfirmed(ctx, v.Home, v.ConfirmURI); err != nil {
		logger.From(ctx, logger.AmfLog).Info("the home AUSF did not take the failed authentication", zap.String("roaming_partner", v.PartnerName), zap.Error(err))
	}
}

func (amf *AMF) homeIdentity(ctx context.Context, imsi string, kind sepp.CallbackKind) (string, string, wire.Guami, error) {
	info, err := amf.OperatorInfo(ctx)
	if err != nil {
		return "", "", wire.Guami{}, err
	}

	op := wire.PlmnIDFromModel(*info.Guami.PlmnID)
	seppID := amf.DBInstance.AMFPointer()

	callback, err := sepp.Callback{NF: sepp.CallbackAMF, SEPPID: seppID, Supi: "imsi-" + imsi, Kind: kind}.URI(op)
	if err != nil {
		return "", "", wire.Guami{}, err
	}

	guami, err := wire.GuamiFromModel(*info.Guami)
	if err != nil {
		return "", "", wire.Guami{}, err
	}

	return roaming.NFInstanceID(sepp.CallbackHost(sepp.CallbackAMF, seppID, op)), callback, guami, nil
}

func (amf *AMF) RequestHomeRegistration(ctx context.Context, ue *UeContext, conn *UeConn, initial bool, done func(context.Context, error)) {
	v, visitor := ue.Visitor()
	if !visitor || (v.Registered && v.Subscription != nil) {
		done(ctx, nil)
		return
	}

	supi := ue.Supi()
	if !supi.IsIMSI() {
		done(ctx, ErrNotVisitor)
		return
	}

	imsi := supi.IMSI()
	pei := ue.Imei().String()

	var err error

	amf.awaitHome(ctx, ue, conn, imsi, "Nudm_UECM_Registration", func(ctx context.Context) {
		err = amf.registerAtHome(ctx, ue, imsi, pei, initial)
	}, func(ctx context.Context) {
		done(ctx, err)
	})
}

func (amf *AMF) registerAtHome(ctx context.Context, ue *UeContext, imsi, pei string, initial bool) error {
	if err := amf.checkVisitorCapacity(ctx, imsi); err != nil {
		return err
	}

	v, ok := ue.Visitor()
	if !ok {
		return ErrNotVisitor
	}

	if err := amf.waitN32(ctx, v); err != nil {
		return err
	}

	instanceID, deregCallback, guami, err := amf.homeIdentity(ctx, imsi, sepp.CallbackUECMDereg)
	if err != nil {
		return err
	}

	_, changeCallback, _, err := amf.homeIdentity(ctx, imsi, sepp.CallbackSDMChange)
	if err != nil {
		return err
	}

	supi := "imsi-" + imsi
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPIFromIMSI(imsi), zap.String("roaming_partner", v.PartnerName))

	if !v.Registered {
		reg := wire.Amf3GppAccessRegistration{
			AmfInstanceID:    instanceID,
			DeregCallbackURI: deregCallback,
			Guami:            &guami,
			RatType:          wire.RatNR,
			ImsVoPs:          wire.ImsVoPsHomogeneousNonSupport,
		}

		if wire.ValidPei(pei) {
			reg.Pei = pei
		}

		if initial {
			reg.InitialRegistrationInd = &initial
		}

		ue.updateVisitor(func(cur *roaming.Visitor5G) {
			cur.Registered = true
			cur.Guami = &guami
		})

		if err := amf.HomeNetwork.Register(ctx, v.Home, supi, reg); err != nil {
			if _, refused := roaming.ProblemOf(err); refused {
				ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Registered = false })
			}

			return err
		}

		log.Info("registered the visitor at its home UDM")
	}

	data, err := amf.HomeNetwork.SubscriptionData(ctx, v.Home, supi)
	if err != nil {
		return err
	}

	if r := homeSubscriptionRestriction(data.AmData); r != nil {
		return r
	}

	sor, sorErr := roaming.SoRFromHome(sorInfoOf(data.AmData))
	if sorErr != nil {
		log.Warn("ignoring the steering of roaming information of the home network", zap.Error(sorErr))
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) {
		cur.Subscription = &data
		cur.SoR = sor
	})

	if v.SDM == nil {
		amf.subscribeAtHome(ctx, ue, v, supi, instanceID, changeCallback)
	}

	return nil
}

func (amf *AMF) subscribeAtHome(ctx context.Context, ue *UeContext, v roaming.Visitor5G, supi, instanceID, callback string) {
	serving := v.Home.Serving
	implicit := true

	sub, err := amf.HomeNetwork.Subscribe(ctx, v.Home, supi, wire.SdmSubscription{
		NfInstanceID:          instanceID,
		ImplicitUnsubscribe:   implicit,
		CallbackReference:     callback,
		MonitoredResourceUris: []string{supi + "/am-data", supi + "/smf-select-data"},
		PlmnID:                &serving,
	})
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to subscribe to the visitor's home subscription data; changes will apply at its next registration",
			logger.SUPI(supi), zap.Error(err))

		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SDM = &sub })
}

func sorInfoOf(am *wire.AccessAndMobilitySubscriptionData) *wire.SorInfo {
	if am == nil {
		return nil
	}

	return am.SorInfo
}

func homeSubscriptionRestriction(am *wire.AccessAndMobilitySubscriptionData) *Rejection {
	switch {
	case am == nil:
		return reject(fgs.GMMCauseN1ModeNotAllowed, "the home network returned no access and mobility subscription")
	case am.CoreNetworkRestricted(wire.CoreNetwork5GC):
		return reject(fgs.GMMCauseN1ModeNotAllowed, "5GC is restricted by the home subscription")
	case am.RATRestricted(wire.RatNR):
		return reject(fgs.GMMCauseNoSuitableCellsInTrackingArea, "NR is restricted by the home subscription")
	case am.RoamingAccessDenied():
		return reject(fgs.GMMCausePLMNNotAllowed, "the home subscription does not allow access to this network")
	}

	return nil
}

func AuthenticationRejection(err error) (*Rejection, bool) {
	var r *Rejection
	if errors.As(err, &r) {
		return r, true
	}

	if errors.Is(err, roaming.ErrUnsupportedAuthType) {
		return reject(fgs.GMMCauseN1ModeNotAllowed, err.Error()), true
	}

	if errors.Is(err, ErrHomeSUPIMismatch) {
		return &Rejection{AuthReject: true, Reason: err.Error()}, true
	}

	pd, ok := roaming.ProblemOf(err)
	if !ok {
		return nil, false
	}

	switch {
	case pd.Status == http.StatusForbidden && (pd.Cause == wire.CauseServingNetworkNotAuthorized || pd.Cause == ""):
		return reject(fgs.GMMCausePLMNNotAllowed, pd.Error()), true
	case pd.Status == http.StatusForbidden, pd.Status == http.StatusNotFound, pd.Status == http.StatusNotImplemented,
		pd.Status == http.StatusBadRequest && (pd.Cause == wire.CauseInvalidSchemeOutput || pd.Cause == wire.CauseInvalidHNPublicKeyIdentifier):
		return &Rejection{AuthReject: true, Reason: pd.Error()}, true
	}

	return nil, false
}

func ConfirmationRejection(err error) (*Rejection, bool) {
	if errors.Is(err, wire.ErrAuthenticationFailed) {
		return reject(fgs.GMMCauseIllegalUE, "the home AUSF did not authenticate the UE"), true
	}

	return AuthenticationRejection(err)
}

func HomeRegistrationRejection(err error) (*Rejection, bool) {
	var r *Rejection
	if errors.As(err, &r) {
		return r, true
	}

	pd, ok := roaming.ProblemOf(err)
	if !ok {
		return nil, false
	}

	switch pd.Status {
	case http.StatusForbidden:
		switch pd.Cause {
		case wire.CauseUnknown5GSSubscription:
			return reject(fgs.GMMCauseN1ModeNotAllowed, pd.Error()), true
		case wire.CauseAccessNotAllowed, wire.CauseRATNotAllowed:
			return reject(fgs.GMMCauseNoSuitableCellsInTrackingArea, pd.Error()), true
		case wire.CauseNoPSSubscription:
			return reject(fgs.GMMCauseServicesNotAllowed, pd.Error()), true
		case wire.CauseReauthenticationRequired:
			return nil, false
		default:
			return reject(fgs.GMMCausePLMNNotAllowed, pd.Error()), true
		}
	case http.StatusNotFound:
		switch pd.Cause {
		case wire.CauseContextNotFound:
			return reject(fgs.GMMCauseUEIdentityCannotBeDerived, pd.Error()), true
		case wire.CauseUserNotFound:
			return reject(fgs.GMMCauseIllegalUE, pd.Error()), true
		default:
			return reject(fgs.GMMCauseN1ModeNotAllowed, pd.Error()), true
		}
	case http.StatusUnprocessableEntity:
		return reject(fgs.GMMCauseProtocolErrorUnspecified, pd.Error()), true
	}

	return nil, false
}

func IsReauthenticationRequired(err error) bool {
	pd, ok := roaming.ProblemOf(err)
	return ok && pd.Status == http.StatusForbidden && pd.Cause == wire.CauseReauthenticationRequired
}

func (amf *AMF) SoRContainer(ue *UeContext) []byte {
	v, ok := ue.Visitor()
	if !ok || v.SoR == nil {
		return nil
	}

	return v.SoR.Container
}

func (amf *AMF) AcknowledgeSoR(ctx context.Context, ue *UeContext, container []byte) {
	v, ok := ue.Visitor()
	if !ok || v.SoR == nil || !v.SoR.AckRequested || amf.HomeNetwork == nil {
		return
	}

	supi := ue.Supi()

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SoR = nil })

	if len(container) == 0 || !supi.IsIMSI() {
		logger.From(ctx, logger.AmfLog).Info("the UE did not acknowledge the steering of roaming information")
		return
	}

	ack, err := roaming.SoRAcknowledgement(container, v.SoR.ProvisioningTime)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("malformed steering of roaming acknowledgement", zap.Error(err))
		return
	}

	ctx = context.WithoutCancel(ctx)
	imsi := supi.IMSI()

	amf.home.Go(imsi, func() {
		if err := amf.HomeNetwork.AcknowledgeSoR(ctx, v.Home, "imsi-"+imsi, ack); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to relay the steering of roaming acknowledgement to the home UDM", logger.SUPIFromIMSI(imsi), zap.Error(err))
		}
	})
}

func (amf *AMF) purgeVisitor(ue *UeContext, supi etsi.SUPI) bool {
	if !supi.IsIMSI() || amf.HomeNetwork == nil {
		return false
	}

	ue.mu.Lock()
	v := ue.visitor
	ue.mu.Unlock()

	if v == nil || (!v.Registered && v.SDM == nil && v.ConfirmURI == "") {
		return false
	}

	snapshot := *v
	imsi := supi.IMSI()

	amf.home.Go(imsi, func() {
		amf.purgeAtHome(context.Background(), imsi, snapshot)
		amf.purgeRegistration(supi)
	})

	return true
}

func (amf *AMF) purgeAtHome(ctx context.Context, imsi string, v roaming.Visitor5G) {
	log := logger.AmfLog.With(logger.SUPIFromIMSI(imsi), zap.String("roaming_partner", v.PartnerName))

	if v.SDM != nil {
		if err := amf.HomeNetwork.Unsubscribe(ctx, v.Home, v.SDM.URI); err != nil {
			log.Info("failed to unsubscribe from the visitor's home subscription data", zap.Error(err))
		}
	}

	if amf.HoldsUE(imsi) {
		return
	}

	if v.Registered && v.Guami != nil {
		if err := amf.HomeNetwork.Deregister(ctx, v.Home, "imsi-"+imsi, *v.Guami); err != nil {
			log.Warn("failed to purge the visitor at its home UDM", zap.Error(err))
			return
		}

		log.Info("purged the visitor at its home UDM")
	}

	switch {
	case v.ConfirmURI == "":
	case v.Confirmed:
		if err := amf.HomeNetwork.RemoveAuthentication(ctx, v.Home, v.ConfirmURI); err != nil {
			log.Info("failed to remove the authentication result at the home AUSF", zap.Error(err))
		}
	default:
		amf.unconfirm(ctx, v)
	}
}

func (amf *AMF) SettleHomeNetwork() {
	amf.home.Settle()
}
