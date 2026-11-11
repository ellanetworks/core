// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
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
	ErrNoHPLMNClient     = errors.New("amf: this node cannot reach home networks")
	ErrNotVisitor        = errors.New("amf: no visitor context for the SUPI")
	ErrHPLMNSUPIMismatch = errors.New("amf: the home network authenticated a SUPI outside the partner's PLMN")
	ErrReauthenticate    = errors.New("amf: the home network requires re-authentication")
)

type RoamingStore interface {
	ListRoamingPartners(ctx context.Context) ([]db.RoamingPartner, error)
	CountSubscribersAndVisitors(ctx context.Context) (int, error)
	SetVisitorMSISDN(ctx context.Context, imsi, msisdn string) error
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

func (amf *AMF) VisitorHPLMN(imsi string) (roaming.Visitor5G, bool) {
	ue, ok := amf.lookupUeByIMSI(imsi)
	if !ok {
		return roaming.Visitor5G{}, false
	}

	return ue.Visitor()
}

func hplmnOfUE(ue *UeContext) (string, string, bool) {
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

	prefix, routingIndicator, ok := hplmnOfUE(ue)
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

	if r := admissionRejection(partner); r != nil {
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

	if supi := ue.Supi(); supi.IsIMSI() {
		visitor.HPLMN.Supi = "imsi-" + supi.IMSI()
	}

	if current, ok := ue.Visitor(); ok && current.PartnerID == visitor.PartnerID && current.HPLMN.PLMN == visitor.HPLMN.PLMN {
		if visitor.HPLMN.RoutingIndicator == "" {
			visitor.HPLMN.RoutingIndicator = current.HPLMN.RoutingIndicator
		}

		if visitor.HPLMN.Supi == "" || visitor.HPLMN.Supi == current.HPLMN.Supi {
			visitor.HPLMN.Supi = current.HPLMN.Supi
			visitor.HPLMN.UDMGroupID = current.HPLMN.UDMGroupID
			visitor.HPLMN.UDMInstanceID = current.HPLMN.UDMInstanceID
		}

		current.HPLMN = visitor.HPLMN
		current.ConfirmURI = ""
		visitor = current
	}

	ue.setVisitor(&visitor)

	logger.From(ctx, logger.AmfLog).Info("inbound roamer", zap.String("roaming_partner", partner.Name), zap.String("home_plmn", visitor.HPLMN.PLMN.String()))

	return nil
}

func admissionRejection(partner db.RoamingPartner) *Rejection {
	if partner.SEPPFQDN == nil {
		return reject(fgs.GMMCauseN1ModeNotAllowed, "the roaming partner has no 5G agreement")
	}

	if !partner.Withdrawing(time.Now()) {
		return nil
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

func (amf *AMF) awaitHPLMN(ctx context.Context, ue *UeContext, conn *UeConn, key, procedure string, job, resume func(context.Context)) {
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
	if amf.HPLMN == nil {
		return fmt.Errorf("%w: %w", roaming.ErrHPLMNUnreachable, ErrNoHPLMNClient)
	}

	if err := amf.HPLMN.WaitN32(ctx, v.PartnerID, N32Wait); err != nil {
		return fmt.Errorf("%w: %w", roaming.ErrHPLMNUnreachable, err)
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
		r, err := hplmnResync(conn, resync)
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

	amf.awaitHPLMN(ctx, ue, conn, authKey(ue), "Nausf_UEAuthentication_Authenticate", func(ctx context.Context) {
		if err = amf.waitN32(ctx, v); err != nil {
			return
		}

		if v.ConfirmURI != "" {
			amf.unconfirm(ctx, v)
		}

		var c roaming.Challenge

		c, err = amf.HPLMN.Authenticate(ctx, v.HPLMN, id, resyncInfo)
		if err != nil {
			return
		}

		ue.updateVisitor(func(cur *roaming.Visitor5G) {
			cur.ConfirmURI = c.ConfirmURI
			cur.Confirmed = false
		})

		result = &ausf.AuthResult{Rand: hex.EncodeToString(c.RAND[:]), Autn: hex.EncodeToString(c.AUTN[:]), HxresStar: hex.EncodeToString(c.HXRESStar[:])}
	}, func(ctx context.Context) {
		amf.noteHPLMNFailure(ctx, v, HPLMNAuthenticate, err, AuthenticationRejection)
		done(ctx, result, err)
	})
}

func hplmnResync(conn *UeConn, resync *ausf.ResyncInfo) (*wire.ResynchronizationInfo, error) {
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

	amf.awaitHPLMN(ctx, ue, conn, authKey(ue), "Nausf_UEAuthentication_Authenticate (confirmation)", func(ctx context.Context) {
		var (
			s string
			k [32]byte
		)

		s, k, err = amf.HPLMN.Confirm(ctx, v.HPLMN, v.ConfirmURI, id, resStar)
		if err != nil {
			return
		}

		imsi, ok := wire.IMSIFromSupi(s)
		if !ok || !strings.HasPrefix(imsi, v.HPLMN.PLMN.Mcc+v.HPLMN.PLMN.Mnc) {
			err = fmt.Errorf("%w: %s", ErrHPLMNSUPIMismatch, s)
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

			if cur.HPLMN.Supi != s {
				cur.HPLMN.Supi = s
				cur.HPLMN.UDMGroupID = ""
				cur.HPLMN.UDMInstanceID = ""
			}
		})
	}, func(ctx context.Context) {
		amf.noteHPLMNFailure(ctx, v, HPLMNConfirm, err, ConfirmationRejection)
		done(ctx, supi, kseaf, err)
	})
}

func (amf *AMF) AuthenticationFailed(ctx context.Context, ue *UeContext) {
	v, ok := ue.Visitor()
	if !ok || v.ConfirmURI == "" || amf.HPLMN == nil {
		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.ConfirmURI = "" })

	ctx = context.WithoutCancel(ctx)

	amf.home.Go(authKey(ue), func() { amf.unconfirm(ctx, v) })
}

func (amf *AMF) unconfirm(ctx context.Context, v roaming.Visitor5G) {
	if err := amf.HPLMN.Unconfirmed(ctx, v.HPLMN, v.ConfirmURI); err != nil {
		logger.From(ctx, logger.AmfLog).Info("the home AUSF did not take the failed authentication", zap.String("roaming_partner", v.PartnerName), zap.Error(err))
	}
}

func (amf *AMF) hplmnIdentity(ctx context.Context, imsi string, kind sepp.CallbackKind) (string, string, wire.Guami, error) {
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

func (amf *AMF) RequestHPLMNRegistration(ctx context.Context, ue *UeContext, conn *UeConn, initial bool, done func(context.Context, error)) {
	v, visitor := ue.Visitor()
	if !visitor {
		done(ctx, nil)
		return
	}

	if initial {
		ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SoR = nil })
	}

	retained := v.Registered && v.HPLMN.UDMInstanceID != "" && v.Subscription != nil
	if retained && (!initial || !sorUpdateRequested(v.AccessAndMobility(), conn.RegistrationType5GS)) {
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

	amf.awaitHPLMN(ctx, ue, conn, imsi, "Nudm_UECM_Registration", func(ctx context.Context) {
		if retained {
			amf.updateSoR(ctx, ue, v, imsi)
			return
		}

		err = amf.registerAtHPLMN(ctx, ue, imsi, pei, initial)
	}, func(ctx context.Context) {
		amf.noteHPLMNFailure(ctx, v, HPLMNRegister, err, HPLMNRegistrationRejection)
		done(ctx, err)
	})
}

func sorUpdateRequested(am *wire.AccessAndMobilitySubscriptionData, regType fgs.RegistrationType) bool {
	if am == nil {
		return false
	}

	switch regType {
	case fgs.RegistrationTypeInitial:
		return slices.Contains(am.SorUpdateIndicatorList, wire.SorUpdateInitialRegistration)
	case fgs.RegistrationTypeEmergency:
		return slices.Contains(am.SorUpdateIndicatorList, wire.SorUpdateEmergencyRegistration)
	}

	return false
}

func (amf *AMF) updateSoR(ctx context.Context, ue *UeContext, v roaming.Visitor5G, imsi string) {
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPIFromIMSI(imsi), zap.String("roaming_partner", v.PartnerName))

	info, err := amf.HPLMN.UpdateSoR(ctx, v.HPLMN, "imsi-"+imsi)
	if err != nil {
		log.Warn("the home UDM did not update the steering of roaming information", zap.Error(err))
		return
	}

	sor, err := roaming.SoRFromHPLMN(&info)
	if err != nil {
		log.Warn("ignoring the steering of roaming information of the home network", zap.Error(err))
		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SoR = sor })
}

func (amf *AMF) registerAtHPLMN(ctx context.Context, ue *UeContext, imsi, pei string, initial bool) error {
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

	instanceID, deregCallback, guami, err := amf.hplmnIdentity(ctx, imsi, sepp.CallbackUECMDereg)
	if err != nil {
		return err
	}

	_, changeCallback, _, err := amf.hplmnIdentity(ctx, imsi, sepp.CallbackSDMChange)
	if err != nil {
		return err
	}

	supi := "imsi-" + imsi
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPIFromIMSI(imsi), zap.String("roaming_partner", v.PartnerName))

	if !v.Registered || v.HPLMN.UDMInstanceID == "" {
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

		v.HPLMN.Supi = supi

		binding, err := amf.HPLMN.Register(ctx, v.HPLMN, supi, reg)
		if err != nil {
			_, refused := roaming.ProblemOf(err)

			ue.updateVisitor(func(cur *roaming.Visitor5G) {
				cur.HPLMN.UDMInstanceID = ""
				cur.Registered = cur.Registered && !refused
			})

			return hplmnFailure(HPLMNRegister, err)
		}

		v.HPLMN.UDMInstanceID, v.HPLMN.UDMGroupID = binding.InstanceID, binding.GroupID

		ue.updateVisitor(func(cur *roaming.Visitor5G) {
			cur.HPLMN.Supi = supi
			cur.HPLMN.UDMInstanceID, cur.HPLMN.UDMGroupID = binding.InstanceID, binding.GroupID
		})

		log.Info("registered the visitor at its home UDM", zap.String("udm_instance", binding.InstanceID), zap.String("udm_group", binding.GroupID))
	}

	data, err := amf.HPLMN.SubscriptionData(ctx, v.HPLMN, supi)
	if err != nil {
		return hplmnFailure(HPLMNSubscriptionData, err)
	}

	if r := hplmnSubscriptionRestriction(data.AmData); r != nil {
		return r
	}

	sor, sorErr := roaming.SoRFromHPLMN(sorInfoOf(data.AmData))
	if sorErr != nil {
		log.Warn("ignoring the steering of roaming information of the home network", zap.Error(sorErr))
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) {
		cur.Subscription = &data
		cur.SoR = sor
	})

	if v.SDM == nil {
		amf.subscribeAtHPLMN(ctx, ue, v, supi, instanceID, changeCallback)
	}

	return nil
}

func (amf *AMF) subscribeAtHPLMN(ctx context.Context, ue *UeContext, v roaming.Visitor5G, supi, instanceID, callback string) {
	serving := v.HPLMN.Serving
	implicit := true

	sub, err := amf.HPLMN.Subscribe(ctx, v.HPLMN, supi, wire.SdmSubscription{
		NfInstanceID:          instanceID,
		ImplicitUnsubscribe:   implicit,
		CallbackReference:     callback,
		MonitoredResourceUris: []string{wire.SDMAmDataPath(supi), wire.SDMSmfSelectDataPath(supi)},
		PlmnID:                &serving,
	})
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to subscribe to the visitor's home subscription data; changes will apply at its next registration",
			logger.SUPI(supi), zap.Error(err))

		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SDM = &sub })
}

func (amf *AMF) syncVisitorMSISDN(ctx context.Context, ue *UeContext) {
	v, ok := ue.Visitor()
	supi := ue.Supi()

	if !ok || !supi.IsIMSI() || v.AccessAndMobility() == nil {
		return
	}

	msisdn, _ := v.AccessAndMobility().MSISDN()

	if err := amf.Roaming.SetVisitorMSISDN(ctx, supi.IMSI(), msisdn); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to store the visitor's MSISDN from its home subscription", logger.SUPI(supi.String()), zap.Error(err))
	}
}

func sorInfoOf(am *wire.AccessAndMobilitySubscriptionData) *wire.SorInfo {
	if am == nil {
		return nil
	}

	return am.SorInfo
}

func hplmnSubscriptionRestriction(am *wire.AccessAndMobilitySubscriptionData) *Rejection {
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

func (amf *AMF) SoRContainer(ue *UeContext) []byte {
	v, ok := ue.Visitor()
	if !ok || v.SoR == nil || v.SoR.Delivered {
		return nil
	}

	return v.SoR.Container
}

func (amf *AMF) AcknowledgeSoR(ctx context.Context, ue *UeContext, container []byte) {
	v, ok := ue.Visitor()
	if !ok || v.SoR == nil || v.SoR.Delivered {
		return
	}

	sor := *v.SoR

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SoR = nil })

	if !sor.AckRequested {
		return
	}

	if len(container) == 0 {
		logger.From(ctx, logger.AmfLog).Info("the UE did not acknowledge the steering of roaming information")
		return
	}

	amf.relaySoRAck(ctx, ue, v, sor, container)
}

func (amf *AMF) relaySoRAck(ctx context.Context, ue *UeContext, v roaming.Visitor5G, sor roaming.SoR, container []byte) {
	supi := ue.Supi()
	if !supi.IsIMSI() || amf.HPLMN == nil {
		return
	}

	ack, err := roaming.SoRAcknowledgement(container, sor.ProvisioningTime)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("malformed steering of roaming acknowledgement", zap.Error(err))
		return
	}

	ctx = context.WithoutCancel(ctx)
	imsi := supi.IMSI()

	amf.home.Go(imsi, func() {
		if err := amf.HPLMN.AcknowledgeSoR(ctx, v.HPLMN, "imsi-"+imsi, ack); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to relay the steering of roaming acknowledgement to the home UDM", logger.SUPIFromIMSI(imsi), zap.Error(err))
		}
	})
}

func (amf *AMF) purgeVisitor(ue *UeContext, supi etsi.SUPI) bool {
	if !supi.IsIMSI() || amf.HPLMN == nil {
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
		amf.purgeAtHPLMN(context.Background(), imsi, snapshot)
		amf.purgeRegistration(supi)
	})

	return true
}

func (amf *AMF) purgeAtHPLMN(ctx context.Context, imsi string, v roaming.Visitor5G) {
	log := logger.AmfLog.With(logger.SUPIFromIMSI(imsi), zap.String("roaming_partner", v.PartnerName))

	if v.SDM != nil {
		if err := amf.HPLMN.Unsubscribe(ctx, v.HPLMN, v.SDM.URI); err != nil {
			log.Info("failed to unsubscribe from the visitor's home subscription data", zap.Error(err))
		}
	}

	if amf.HoldsUE(imsi) {
		return
	}

	if v.Registered && v.Guami != nil {
		if err := amf.HPLMN.Deregister(ctx, v.HPLMN, "imsi-"+imsi, *v.Guami); err != nil {
			log.Warn("failed to purge the visitor at its home UDM", zap.Error(err))
			return
		}

		log.Info("purged the visitor at its home UDM")
	}

	switch {
	case v.ConfirmURI == "":
	case v.Confirmed:
		if err := amf.HPLMN.RemoveAuthentication(ctx, v.HPLMN, v.ConfirmURI); err != nil {
			log.Info("failed to remove the authentication result at the home AUSF", zap.Error(err))
		}
	default:
		amf.unconfirm(ctx, v)
	}
}

func (amf *AMF) SettleHPLMN() {
	amf.home.Settle()
}
