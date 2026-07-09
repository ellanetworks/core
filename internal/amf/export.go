// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

// UeContextExport is the JSON-serializable export of a single UE's AMF state.
type UeContextExport struct {
	Identity      UEIdentityExport            `json:"identity"`
	State         UEStateExport               `json:"state"`
	Security      UESecurityExport            `json:"security"`
	Location      UELocationExport            `json:"location"`
	Subscription  UESubscriptionExport        `json:"subscription"`
	PDUSessions   map[string]PDUSessionExport `json:"pdu_sessions"`
	Registration  UERegistrationExport        `json:"registration"`
	Timers        UETimersExport              `json:"timers"`
	LastActivity  UELastActivityExport        `json:"last_activity"`
	RANConnection *RANConnectionExport        `json:"ran_connection,omitempty"`
}

type UEIdentityExport struct {
	Supi    string        `json:"supi"`
	Pei     string        `json:"pei,omitempty"`
	PlmnID  models.PlmnID `json:"plmn_id"`
	Guti    string        `json:"guti,omitempty"`
	OldGuti string        `json:"old_guti,omitempty"`
	Tmsi    string        `json:"tmsi,omitempty"`
	OldTmsi string        `json:"old_tmsi,omitempty"`
	Suci    string        `json:"suci,omitempty"`
}

type UEStateExport struct {
	GMMState                 string   `json:"gmm_state"`
	OngoingProcedures        []string `json:"ongoing_procedures"`
	SecurityContextAvailable bool     `json:"security_context_available"`
}

// UESecurityExport contains non-sensitive security context info for a UE.
type UESecurityExport struct {
	CipheringAlgorithm string       `json:"ciphering_algorithm,omitempty"`
	IntegrityAlgorithm string       `json:"integrity_algorithm,omitempty"`
	NgKsi              models.NgKsi `json:"ng_ksi"`
}

type UELocationExport struct {
	Current          models.UserLocation `json:"current"`
	Tai              models.Tai          `json:"tai"`
	RegistrationArea []models.Tai        `json:"registration_area"`
}

type UESubscriptionExport struct {
	AllowedNssai []models.Snssai `json:"allowed_nssai,omitempty"`
	Ambr         *models.Ambr    `json:"ambr,omitempty"`
}

type PDUSessionExport struct {
	Ref                            string            `json:"ref"`
	PDUSessionID                   uint8             `json:"pdu_session_id"`
	PDUSessionType                 uint8             `json:"pdu_session_type,omitempty"` // NAS PDU session type: 1 IPv4 / 2 IPv6 / 3 IPv4v6
	Snssai                         *models.Snssai    `json:"snssai,omitempty"`
	Inactive                       bool              `json:"inactive"`
	DNN                            string            `json:"dnn,omitempty"`
	PDUIPV4Address                 string            `json:"pduIpv4Address,omitempty"`
	PDUIPV6Prefix                  string            `json:"pduIpv6Prefix,omitempty"`
	PDUSessionReleaseDueToDupPduID bool              `json:"release_due_to_dup_id,omitempty"`
	PolicyData                     *PolicyDataExport `json:"policy_data,omitempty"`
	Tunnel                         *TunnelExport     `json:"tunnel,omitempty"`
	PFCPLocalSEID                  *uint64           `json:"pfcp_local_seid,omitempty"`
}

type PolicyDataExport struct {
	Ambr    *models.Ambr    `json:"Ambr,omitempty"`
	QosData *models.QosData `json:"QosData,omitempty"`
}

type TunnelExport struct {
	ANIPAddress string `json:"an_ip_address,omitempty"`
	ANTEID      uint32 `json:"an_teid,omitempty"`
}

type UERegistrationExport struct {
	Type             uint8 `json:"type"`
	IdentityTypeUsed uint8 `json:"identity_type_used"`
	Retransmission   bool  `json:"retransmission"`
	ResyncTried      bool  `json:"resync_tried"`
}

type TimerStatusExport struct {
	Active      bool  `json:"active"`
	ExpireCount int32 `json:"expire_count,omitempty"`
	MaxRetries  int32 `json:"max_retries,omitempty"`
}

type UETimersExport struct {
	T3512ValueSeconds int64             `json:"t3512_value_seconds"`
	T3502ValueSeconds int64             `json:"t3502_value_seconds"`
	Paging            TimerStatusExport `json:"paging"`
	NASGuard          TimerStatusExport `json:"nas_guard"`
	NASGuardProcedure string            `json:"nas_guard_procedure"`
	MobileReachable   TimerStatusExport `json:"mobile_reachable"`
	ImplicitDereg     TimerStatusExport `json:"implicit_deregistration"`
}

type UELastActivityExport struct {
	Timestamp time.Time `json:"timestamp"`
	RadioNode string    `json:"radio_node,omitempty"`
}

type RANConnectionExport struct {
	RanUeNgapID int64      `json:"ran_ue_ngap_id"`
	AmfUeNgapID int64      `json:"amf_ue_ngap_id"`
	RanTai      models.Tai `json:"ran_tai"`
	RadioName   string     `json:"radio_name,omitempty"`
}

func copyPtr[T any](src *T) *T {
	if src == nil {
		return nil
	}

	cp := *src

	return &cp
}

// copyUserLocation returns a copy of a UserLocation with each pointer field
// independently owned, preventing aliasing with the source struct.
func copyUserLocation(loc models.UserLocation) models.UserLocation {
	out := loc

	out.NrLocation = copyPtr(loc.NrLocation)
	if out.NrLocation != nil {
		out.NrLocation.Tai = copyPtr(loc.NrLocation.Tai)
		if out.NrLocation.Tai != nil {
			out.NrLocation.Tai.PlmnID = copyPtr(loc.NrLocation.Tai.PlmnID)
		}

		out.NrLocation.Ncgi = copyPtr(loc.NrLocation.Ncgi)
		if out.NrLocation.Ncgi != nil {
			out.NrLocation.Ncgi.PlmnID = copyPtr(loc.NrLocation.Ncgi.PlmnID)
		}

		out.NrLocation.UeLocationTimestamp = copyPtr(loc.NrLocation.UeLocationTimestamp)
	}

	out.EutraLocation = copyPtr(loc.EutraLocation)
	if out.EutraLocation != nil {
		out.EutraLocation.Tai = copyPtr(loc.EutraLocation.Tai)
		if out.EutraLocation.Tai != nil {
			out.EutraLocation.Tai.PlmnID = copyPtr(loc.EutraLocation.Tai.PlmnID)
		}

		out.EutraLocation.Ecgi = copyPtr(loc.EutraLocation.Ecgi)
		if out.EutraLocation.Ecgi != nil {
			out.EutraLocation.Ecgi.PlmnID = copyPtr(loc.EutraLocation.Ecgi.PlmnID)
		}

		out.EutraLocation.UeLocationTimestamp = copyPtr(loc.EutraLocation.UeLocationTimestamp)
	}

	out.N3gaLocation = copyPtr(loc.N3gaLocation)
	if out.N3gaLocation != nil {
		out.N3gaLocation.N3gppTai = copyPtr(loc.N3gaLocation.N3gppTai)
		if out.N3gaLocation.N3gppTai != nil {
			out.N3gaLocation.N3gppTai.PlmnID = copyPtr(loc.N3gaLocation.N3gppTai.PlmnID)
		}
	}

	return out
}

func policyDataFromSMF(src *smf.Policy) *PolicyDataExport {
	if src == nil {
		return nil
	}

	ambr := src.Ambr
	qosData := src.QosData

	return &PolicyDataExport{
		Ambr: &ambr,
		QosData: &models.QosData{
			QFI:    qosData.QFI,
			Var5qi: qosData.Var5qi,
			Arp:    copyPtr(qosData.Arp),
		},
	}
}

// timerStatus reports a guard's status; a nil guard is inactive.
func timerStatus(g *guard.Guard) TimerStatusExport {
	if g == nil {
		return TimerStatusExport{Active: false}
	}

	return TimerStatusExport{
		Active:      g.Active(),
		ExpireCount: g.ExpireTimes(),
		MaxRetries:  g.MaxRetryTimes(),
	}
}

// ExportUEs returns a snapshot of all current UEs in the AMF context.
// Safe to call concurrently with normal AMF operation.
func (amf *AMF) ExportUEs(ctx context.Context) ([]UeContextExport, error) {
	amf.mu.RLock()

	ues := make([]*UeContext, 0, len(amf.UEs))
	for _, ue := range amf.UEs {
		ues = append(ues, ue)
	}

	amf.mu.RUnlock()

	exports := make([]UeContextExport, 0, len(ues))
	if len(ues) == 0 {
		return exports, nil
	}

	// The GUAMI rebuilds each UE's GUTI, a cosmetic field; the stored 5G-TMSI is
	// exported regardless. Fetched once and best-effort: an unreadable operator config
	// leaves the GUTI empty without failing the snapshot.
	var guami *models.Guami

	if amf.DBInstance != nil {
		if operatorInfo, err := amf.OperatorInfo(ctx); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("export: operator info unavailable, omitting GUTI", zap.Error(err))
		} else {
			guami = operatorInfo.Guami
		}
	}

	for _, ue := range ues {
		exports = append(exports, amf.exportUeContext(guami, ue))
	}

	return exports, nil
}

// LookupSubscriber returns the snapshot, live radio name, and PDU sessions of a
// Registered UE by SUPI (ok is false for an unknown or not-yet-Registered UE), so the
// views cannot tear across separate registry lookups. Session detail is built after the
// session refs are copied out from under the UE lock, because it reaches the SMF
// (SMF-delegated sessions, TS 23.501) and must not run under the registry lock.
func (amf *AMF) LookupSubscriber(supi etsi.SUPI) (UESnapshot, string, []PDUSessionExport, bool) {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok || ue.State() != Registered {
		return UESnapshot{}, "", nil, false
	}

	snap := ue.Snapshot()

	// The radio is the UE's live connection (an idle UE reports none), derived here
	// rather than persisted historically.
	radioName := ""
	if conn := ue.Conn(); conn != nil {
		radioName = conn.radioName
	}

	ue.mu.Lock()

	smCopies := make([]smContextCopy, 0, len(ue.SmContextList))
	for _, sc := range ue.SmContextList {
		smCopies = append(smCopies, smContextCopy{
			ref:      sc.Ref,
			snssai:   copyPtr(sc.Snssai),
			inactive: sc.PduSessionInactive,
		})
	}

	ue.mu.Unlock()

	sessions := amf.buildPDUSessions(smCopies)

	result := make([]PDUSessionExport, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, s)
	}

	return snap, radioName, result, true
}

// smContextCopy is a local copy of AMF SmContext fields used to avoid holding the UE lock while querying SMF.
type smContextCopy struct {
	ref      string
	snssai   *models.Snssai
	inactive bool
}

func (amf *AMF) exportUeContext(guami *models.Guami, ue *UeContext) UeContextExport {
	ue.mu.Lock()

	conn := ue.Conn()

	// gutiFor is pure, so rebuilding the GUTI strings under ue.mu takes no amf.mu
	// against the lock order. An unset TMSI yields InvalidGUTI.
	guti, _ := gutiFor(guami, ue.tmsi)
	oldGuti, _ := gutiFor(guami, ue.oldTmsi)

	var (
		ongoing      []string
		regType      uint8
		identityType uint8
		retransmit   bool
		resyncTried  bool
		nasGuard     *guard.Guard
		nasGuardName string
	)

	if conn != nil {
		ongoing = conn.Parent().Procedures().ActiveTypes()
		regType = conn.RegistrationType5GS
		identityType = conn.IdentityTypeUsedForRegistration
		retransmit = conn.RetransmissionOfInitialNASMsg
		resyncTried = conn.resyncTried
		nasGuard = &conn.nasGuard
		nasGuardName = conn.nasGuardProcName()
	}

	export := UeContextExport{
		Identity: UEIdentityExport{
			Supi:    ue.supi.String(),
			Pei:     ue.Imei.String(),
			PlmnID:  ue.PlmnID,
			Guti:    guti.String(),
			OldGuti: oldGuti.String(),
			Tmsi:    ue.tmsi.String(),
			OldTmsi: ue.oldTmsi.String(),
			Suci:    ue.Suci,
		},
		State: UEStateExport{
			GMMState:                 ue.state.String(),
			OngoingProcedures:        ongoing,
			SecurityContextAvailable: ue.secured,
		},
		Security: UESecurityExport{
			CipheringAlgorithm: cipheringAlgName(ue.cipheringAlg),
			IntegrityAlgorithm: integrityAlgName(ue.integrityAlg),
			NgKsi:              ue.ngKsi,
		},
		Location: UELocationExport{
			Current:          copyUserLocation(ue.Location),
			Tai:              ue.Tai,
			RegistrationArea: append([]models.Tai(nil), ue.RegistrationArea...),
		},
		Subscription: UESubscriptionExport{
			AllowedNssai: append([]models.Snssai(nil), ue.AllowedNssai...),
			Ambr:         copyPtr(ue.Ambr),
		},
		Registration: UERegistrationExport{
			Type:             regType,
			IdentityTypeUsed: identityType,
			Retransmission:   retransmit,
			ResyncTried:      resyncTried,
		},
		Timers: UETimersExport{
			T3512ValueSeconds: int64(amf.T3512Value / time.Second),
			T3502ValueSeconds: int64(amf.T3502Value / time.Second),
			Paging:            timerStatus(&ue.pagingTimer),
			NASGuard:          timerStatus(nasGuard),
			NASGuardProcedure: nasGuardName,
			MobileReachable:   timerStatus(&ue.mobileReachableTimer),
			ImplicitDereg:     timerStatus(&ue.implicitDeregistrationTimer),
		},
		LastActivity: UELastActivityExport{
			Timestamp: ue.lastSeenTime(),
		},
	}

	smCopies := make([]smContextCopy, 0, len(ue.SmContextList))
	for _, sc := range ue.SmContextList {
		smCopies = append(smCopies, smContextCopy{
			ref:      sc.Ref,
			snssai:   copyPtr(sc.Snssai),
			inactive: sc.PduSessionInactive,
		})
	}

	if r := ue.active.Load(); r != nil {
		// The last-seen radio is the UE's live connection (an idle UE is on none).
		export.LastActivity.RadioNode = r.radioName

		rc := &RANConnectionExport{
			RanUeNgapID: int64(r.RanUeNgapID),
			AmfUeNgapID: int64(r.AmfUeNgapID),
			RanTai:      r.Tai,
			RadioName:   r.radioName,
		}

		export.RANConnection = rc
	}

	ue.mu.Unlock()

	// Build PDU sessions outside the UE lock to avoid holding two locks at once.
	export.PDUSessions = amf.buildPDUSessions(smCopies)

	return export
}

func (amf *AMF) buildPDUSessions(copies []smContextCopy) map[string]PDUSessionExport {
	result := make(map[string]PDUSessionExport, len(copies))
	smfSessions := amf.Session

	for _, sc := range copies {
		pdu := PDUSessionExport{
			Ref:      sc.ref,
			Snssai:   sc.snssai,
			Inactive: sc.inactive,
		}

		if smfSessions == nil {
			result[sc.ref] = pdu
			continue
		}

		smCtx := smfSessions.GetSession(sc.ref)
		if smCtx != nil {
			smCtx.Mutex.Lock()
			pdu.PDUSessionID = smCtx.PDUSessionID
			pdu.PDUSessionType = smCtx.PDUSessionType
			pdu.DNN = smCtx.Dnn
			pdu.PDUSessionReleaseDueToDupPduID = smCtx.PDUSessionReleaseDueToDupPduID

			if smCtx.PDUIPV4Address != nil {
				pdu.PDUIPV4Address = smCtx.PDUIPV4Address.String()
			}

			if smCtx.PDUIPV6Prefix != nil {
				pdu.PDUIPV6Prefix = smCtx.PDUIPV6Prefix.String()
			}

			pdu.PolicyData = policyDataFromSMF(smCtx.PolicyData)
			if smCtx.Tunnel != nil {
				ipStr := ""
				if smCtx.Tunnel.ANInformation.IPv4Address != nil {
					ipStr = smCtx.Tunnel.ANInformation.IPv4Address.String()
				}

				pdu.Tunnel = &TunnelExport{
					ANIPAddress: ipStr,
					ANTEID:      smCtx.Tunnel.ANInformation.TEID,
				}
			}

			if smCtx.PFCPContext != nil {
				seid := smCtx.PFCPContext.LocalSEID
				pdu.PFCPLocalSEID = &seid
			}

			smCtx.Mutex.Unlock()
		}

		result[sc.ref] = pdu
	}

	return result
}
