// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

// AmfUeExport is the JSON-serializable export of a single UE's AMF state.
type AmfUeExport struct {
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

// UEIdentityExport contains the identifiers for a UE.
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

// UEStateExport contains the current GMM and procedure state of a UE.
type UEStateExport struct {
	GMMState                 string `json:"gmm_state"`
	OngoingProcedure         string `json:"ongoing_procedure"`
	SecurityContextAvailable bool   `json:"security_context_available"`
	MacFailed                bool   `json:"mac_failed"`
}

// UESecurityExport contains non-sensitive security context info for a UE.
type UESecurityExport struct {
	CipheringAlgorithm string       `json:"ciphering_algorithm,omitempty"`
	IntegrityAlgorithm string       `json:"integrity_algorithm,omitempty"`
	NgKsi              models.NgKsi `json:"ng_ksi"`
}

// UELocationExport contains location information for a UE.
type UELocationExport struct {
	Current          models.UserLocation `json:"current"`
	Tai              models.Tai          `json:"tai"`
	Timezone         string              `json:"timezone,omitempty"`
	RegistrationArea []models.Tai        `json:"registration_area"`
}

// UESubscriptionExport contains subscription data for a UE.
type UESubscriptionExport struct {
	AllowedNssai *models.Snssai `json:"allowed_nssai,omitempty"`
	Ambr         *models.Ambr   `json:"ambr,omitempty"`
}

// PDUSessionExport contains the export of a single PDU session.
type PDUSessionExport struct {
	Ref                            string            `json:"ref"`
	Snssai                         *models.Snssai    `json:"snssai,omitempty"`
	Inactive                       bool              `json:"inactive"`
	DNN                            string            `json:"dnn,omitempty"`
	PDUSessionReleaseDueToDupPduID bool              `json:"release_due_to_dup_id,omitempty"`
	PolicyData                     *PolicyDataExport `json:"policy_data,omitempty"`
	Tunnel                         *TunnelExport     `json:"tunnel,omitempty"`
	PFCPLocalSEID                  *uint64           `json:"pfcp_local_seid,omitempty"`
}

// PolicyDataExport is the JSON-serializable QoS policy snapshot for a PDU session.
type PolicyDataExport struct {
	Ambr    *models.Ambr    `json:"Ambr,omitempty"`
	QosData *models.QosData `json:"QosData,omitempty"`
}

// TunnelExport contains the AN tunnel endpoint information for a PDU session.
type TunnelExport struct {
	ANIPAddress string `json:"an_ip_address,omitempty"`
	ANTEID      uint32 `json:"an_teid,omitempty"`
}

// UERegistrationExport contains registration procedure information for a UE.
type UERegistrationExport struct {
	Type                 uint8 `json:"type"`
	IdentityTypeUsed     uint8 `json:"identity_type_used"`
	Retransmission       bool  `json:"retransmission"`
	AuthFailureSyncTimes int   `json:"auth_failure_sync_times"`
}

// TimerStatusExport contains the status of a single 3GPP timer.
type TimerStatusExport struct {
	Active      bool  `json:"active"`
	ExpireCount int32 `json:"expire_count,omitempty"`
	MaxRetries  int32 `json:"max_retries,omitempty"`
}

// UETimersExport contains the status of all 3GPP timers for a UE.
type UETimersExport struct {
	T3512ValueSeconds   int64             `json:"t3512_value_seconds"`
	T3502ValueSeconds   int64             `json:"t3502_value_seconds"`
	T3513Paging         TimerStatusExport `json:"t3513_paging"`
	T3565Notification   TimerStatusExport `json:"t3565_notification"`
	T3560Auth           TimerStatusExport `json:"t3560_auth"`
	T3550Registration   TimerStatusExport `json:"t3550_registration"`
	T3555ConfigUpdate   TimerStatusExport `json:"t3555_config_update"`
	T3522Deregistration TimerStatusExport `json:"t3522_deregistration"`
	MobileReachable     TimerStatusExport `json:"mobile_reachable"`
	ImplicitDereg       TimerStatusExport `json:"implicit_deregistration"`
}

// UELastActivityExport contains the last-seen activity info for a UE.
type UELastActivityExport struct {
	Timestamp time.Time `json:"timestamp"`
	RadioNode string    `json:"radio_node,omitempty"`
}

// RANConnectionExport contains a lightweight summary of the RAN UE connection.
type RANConnectionExport struct {
	RanUeNgapID int64      `json:"ran_ue_ngap_id"`
	AmfUeNgapID int64      `json:"amf_ue_ngap_id"`
	RanTai      models.Tai `json:"ran_tai"`
	RadioName   string     `json:"radio_name,omitempty"`
}

// copyPtr returns a shallow copy of the value pointed to by src, or nil if src is nil.
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

// policyDataFromSMF converts an SMF Policy to the export struct.
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

// timerStatus returns a TimerStatusExport for the given timer. Safe to call with nil.
func timerStatus(t *Timer) TimerStatusExport {
	if t == nil {
		return TimerStatusExport{Active: false}
	}

	return TimerStatusExport{
		Active:      t.IsActive(),
		ExpireCount: t.ExpireTimes(),
		MaxRetries:  t.MaxRetryTimes(),
	}
}

// ExportUEs returns a snapshot of all current UEs in the AMF context.
// It acquires the AMF lock to get the list of UEs, then acquires
// locks per-UE and calls into the SMF singleton for PDU session
// details. Safe to call concurrently with normal AMF operation.
func (amf *AMF) ExportUEs(_ context.Context) ([]AmfUeExport, error) {
	amf.Mutex.Lock()

	ues := make([]*AmfUe, 0, len(amf.UEs))
	for _, ue := range amf.UEs {
		ues = append(ues, ue)
	}

	amf.Mutex.Unlock()

	exports := make([]AmfUeExport, 0, len(ues))
	for _, ue := range ues {
		exports = append(exports, amf.exportAmfUe(ue))
	}

	return exports, nil
}

// GetUEPDUSessions returns the PDU sessions for a single UE identified by SUPI.
// Returns the PDU session exports and true if the UE exists, false otherwise.
// Safe to call concurrently with normal AMF operation.
func (amf *AMF) GetUEPDUSessions(supi etsi.SUPI) ([]PDUSessionExport, bool) {
	ue, ok := amf.FindAMFUEBySupi(supi)
	if !ok {
		return nil, false
	}

	ue.Mutex.Lock()

	smCopies := make([]smContextCopy, 0, len(ue.SmContextList))
	for _, sc := range ue.SmContextList {
		smCopies = append(smCopies, smContextCopy{
			ref:      sc.Ref,
			snssai:   copyPtr(sc.Snssai),
			inactive: sc.PduSessionInactive,
		})
	}

	ue.Mutex.Unlock()

	sessions := amf.buildPDUSessions(smCopies)

	result := make([]PDUSessionExport, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, s)
	}

	return result, true
}

// smContextCopy is a local copy of AMF SmContext fields used to avoid holding the UE lock while querying SMF.
type smContextCopy struct {
	ref      string
	snssai   *models.Snssai
	inactive bool
}

// exportAmfUe builds an AmfUeExport for a single UE.
// It acquires ue.Mutex to copy scalar fields, then queries SMF outside the lock.
func (amf *AMF) exportAmfUe(ue *AmfUe) AmfUeExport {
	ue.Mutex.Lock()

	// Copy all scalar fields while holding the lock.
	export := AmfUeExport{
		Identity: UEIdentityExport{
			Supi:    ue.Supi.String(),
			Pei:     ue.Pei,
			PlmnID:  ue.PlmnID,
			Guti:    ue.Guti.String(),
			OldGuti: ue.OldGuti.String(),
			Tmsi:    ue.Tmsi.String(),
			OldTmsi: ue.OldTmsi.String(),
			Suci:    ue.Suci,
		},
		State: UEStateExport{
			GMMState:                 string(ue.State),
			OngoingProcedure:         string(ue.OnGoing),
			SecurityContextAvailable: ue.SecurityContextAvailable,
			MacFailed:                ue.MacFailed,
		},
		Security: UESecurityExport{
			CipheringAlgorithm: ue.cipheringAlgName(),
			IntegrityAlgorithm: ue.integrityAlgName(),
			NgKsi:              ue.NgKsi,
		},
		Location: UELocationExport{
			Current:          copyUserLocation(ue.Location),
			Tai:              ue.Tai,
			Timezone:         ue.TimeZone,
			RegistrationArea: append([]models.Tai(nil), ue.RegistrationArea...),
		},
		Subscription: UESubscriptionExport{
			AllowedNssai: copyPtr(ue.AllowedNssai),
			Ambr:         copyPtr(ue.Ambr),
		},
		Registration: UERegistrationExport{
			Type:                 ue.RegistrationType5GS,
			IdentityTypeUsed:     ue.IdentityTypeUsedForRegistration,
			Retransmission:       ue.RetransmissionOfInitialNASMsg,
			AuthFailureSyncTimes: ue.AuthFailureCauseSynchFailureTimes,
		},
		Timers: UETimersExport{
			T3512ValueSeconds:   int64(ue.T3512Value / time.Second),
			T3502ValueSeconds:   int64(ue.T3502Value / time.Second),
			T3513Paging:         timerStatus(ue.T3513),
			T3565Notification:   timerStatus(ue.T3565),
			T3560Auth:           timerStatus(ue.T3560),
			T3550Registration:   timerStatus(ue.T3550),
			T3555ConfigUpdate:   timerStatus(ue.T3555),
			T3522Deregistration: timerStatus(ue.T3522),
			MobileReachable:     timerStatus(ue.mobileReachableTimer),
			ImplicitDereg:       timerStatus(ue.implicitDeregistrationTimer),
		},
		LastActivity: UELastActivityExport{
			Timestamp: ue.LastSeenAt,
			RadioNode: ue.LastSeenRadio,
		},
	}

	// Copy SmContextList refs while holding the UE lock.
	smCopies := make([]smContextCopy, 0, len(ue.SmContextList))
	for _, sc := range ue.SmContextList {
		smCopies = append(smCopies, smContextCopy{
			ref:      sc.Ref,
			snssai:   copyPtr(sc.Snssai),
			inactive: sc.PduSessionInactive,
		})
	}

	// Capture RAN UE info while holding the UE lock.
	if ue.RanUe != nil {
		rc := &RANConnectionExport{
			RanUeNgapID: ue.RanUe.RanUeNgapID,
			AmfUeNgapID: ue.RanUe.AmfUeNgapID,
			RanTai:      ue.RanUe.Tai,
		}
		if ue.RanUe.Radio != nil {
			rc.RadioName = ue.RanUe.Radio.Name
		}

		export.RANConnection = rc
	}

	ue.Mutex.Unlock()

	// Build PDU sessions OUTSIDE the UE lock to avoid holding two locks simultaneously.
	export.PDUSessions = amf.buildPDUSessions(smCopies)

	return export
}

// buildPDUSessions enriches AMF SmContext copies with SMF context data.
func (amf *AMF) buildPDUSessions(copies []smContextCopy) map[string]PDUSessionExport {
	result := make(map[string]PDUSessionExport, len(copies))
	smfSessions := amf.Smf

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
			pdu.DNN = smCtx.Dnn
			pdu.PDUSessionReleaseDueToDupPduID = smCtx.PDUSessionReleaseDueToDupPduID

			pdu.PolicyData = policyDataFromSMF(smCtx.PolicyData)
			if smCtx.Tunnel != nil {
				ipStr := ""
				if smCtx.Tunnel.ANInformation.IPAddress != nil {
					ipStr = smCtx.Tunnel.ANInformation.IPAddress.String()
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
