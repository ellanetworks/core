// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
)

// UeContextExport is a snapshot of one UE's MME context for the support bundle.
// Facets with no EPS analog are omitted:
// SUCI, allowed NSSAI and ng-KSI are 5G-only; per-UE location is dropped because
// the MME does not retain the decoded ULI; the full GUTI string is omitted (only
// the M-TMSI is stored per UE), and the NAS-guard procedure name is AMF-only.
type UeContextExport struct {
	Identity       UEIdentityExport               `json:"identity"`
	State          UEStateExport                  `json:"state"`
	Security       UESecurityExport               `json:"security"`
	Subscription   UESubscriptionExport           `json:"subscription"`
	PDNConnections map[string]PDNConnectionExport `json:"pdn_connections"`
	Registration   UERegistrationExport           `json:"registration"`
	Timers         UETimersExport                 `json:"timers"`
	LastActivity   UELastActivityExport           `json:"last_activity"`
	RANConnection  *RANConnectionExport           `json:"ran_connection,omitempty"`
}

type UEIdentityExport struct {
	Supi    string        `json:"supi"`
	Pei     string        `json:"pei,omitempty"`
	PlmnID  models.PlmnID `json:"plmn_id"`
	Tmsi    string        `json:"tmsi,omitempty"`
	OldTmsi string        `json:"old_tmsi,omitempty"`
}

type UEStateExport struct {
	EMMState                 string   `json:"emm_state"`
	OngoingProcedures        []string `json:"ongoing_procedures"`
	SecurityContextAvailable bool     `json:"security_context_available"`
}

// UESecurityExport contains non-sensitive security context info for a UE.
type UESecurityExport struct {
	CipheringAlgorithm string `json:"ciphering_algorithm,omitempty"`
	IntegrityAlgorithm string `json:"integrity_algorithm,omitempty"`
}

type UESubscriptionExport struct {
	Ambr *models.Ambr `json:"ambr,omitempty"`
}

type PDNConnectionExport struct {
	Ebi                    uint8         `json:"ebi"`
	Apn                    string        `json:"apn,omitempty"`
	PdnType                uint8         `json:"pdn_type,omitempty"` // 1 IPv4 / 2 IPv6 / 3 IPv4v6
	UeIPv4Address          string        `json:"ue_ipv4_address,omitempty"`
	UeIPv6Prefix           string        `json:"ue_ipv6_prefix,omitempty"`
	Qci                    uint8         `json:"qci,omitempty"`
	Arp                    uint8         `json:"arp,omitempty"`
	SessionAMBRUplinkBps   uint64        `json:"session_ambr_uplink_bps,omitempty"`
	SessionAMBRDownlinkBps uint64        `json:"session_ambr_downlink_bps,omitempty"`
	Tunnel                 *TunnelExport `json:"tunnel,omitempty"`
}

// TunnelExport is a PDN connection's S1-U endpoints (TS 36.413): the S-GW side the
// eNB sends uplink to, and the eNB side learned at Initial Context Setup.
type TunnelExport struct {
	SgwS1UAddress string `json:"sgw_s1u_address,omitempty"`
	SgwS1UTEID    uint32 `json:"sgw_s1u_teid,omitempty"`
	EnbS1UAddress string `json:"enb_s1u_address,omitempty"`
	EnbS1UTEID    uint32 `json:"enb_s1u_teid,omitempty"`
}

type UERegistrationExport struct {
	CombinedAttach bool `json:"combined_attach"`
	ResyncTried    bool `json:"resync_tried"`
}

type TimerStatusExport struct {
	Active      bool  `json:"active"`
	ExpireCount int32 `json:"expire_count,omitempty"`
	MaxRetries  int32 `json:"max_retries,omitempty"`
}

type UETimersExport struct {
	T3412ValueSeconds int64             `json:"t3412_value_seconds"`
	T3402ValueSeconds int64             `json:"t3402_value_seconds"`
	MobileReachable   TimerStatusExport `json:"mobile_reachable"`
	ImplicitDetach    TimerStatusExport `json:"implicit_detach"`
	NASGuard          TimerStatusExport `json:"nas_guard"`
	Paging            TimerStatusExport `json:"paging"`
}

type UELastActivityExport struct {
	Timestamp time.Time `json:"timestamp"`
	ENBNode   string    `json:"enb_node,omitempty"`
}

type RANConnectionExport struct {
	MMEUES1APID uint32 `json:"mme_ue_s1ap_id"`
	ENBUES1APID uint32 `json:"enb_ue_s1ap_id"`
	ENBName     string `json:"enb_name,omitempty"`
}

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

func copyAmbr(src *models.Ambr) *models.Ambr {
	if src == nil {
		return nil
	}

	cp := *src

	return &cp
}

// ExportUEs returns a snapshot of every UE context in the MME for the support
// bundle. Safe to call concurrently with normal operation.
func (m *MME) ExportUEs(ctx context.Context) ([]UeContextExport, error) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return nil, fmt.Errorf("get operator PLMN: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]UeContextExport, 0, len(m.UEs))
	for _, ue := range m.UEs {
		out = append(out, m.exportUeContext(plmn, ue))
	}

	return out, nil
}

// exportUeContext builds one UE's export. The caller holds m.mu (the eNB-name
// lookup reads m.radios); ue.mu is taken here for the UE's data. Order m.mu → ue.mu.
func (m *MME) exportUeContext(plmn models.PlmnID, ue *UeContext) UeContextExport {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	var ongoing []string
	if ue.procedures != nil {
		ongoing = ue.procedures.ActiveTypes()
	}

	// resyncTried is in-flight auth state on the connection; an idle UE has none.
	resyncTried := false
	if c := ue.Conn(); c != nil {
		resyncTried = c.resyncTried
	}

	export := UeContextExport{
		Identity: UEIdentityExport{
			Supi:    ue.supi.String(),
			Pei:     ue.Imei.IMEI(),
			PlmnID:  plmn,
			Tmsi:    ue.Tmsi().String(),
			OldTmsi: ue.OldTmsi().String(),
		},
		State: UEStateExport{
			EMMState:                 ue.emmState.String(),
			OngoingProcedures:        ongoing,
			SecurityContextAvailable: ue.secured,
		},
		Security: UESecurityExport{
			CipheringAlgorithm: epsCipheringAlgName(ue.cipheringAlg),
			IntegrityAlgorithm: epsIntegrityAlgName(ue.integrityAlg),
		},
		Subscription: UESubscriptionExport{
			Ambr: copyAmbr(ue.Ambr),
		},
		Registration: UERegistrationExport{
			CombinedAttach: ue.CombinedAttach,
			ResyncTried:    resyncTried,
		},
		Timers: UETimersExport{
			T3412ValueSeconds: int64(T3412PeriodicTAU / time.Second),
			T3402ValueSeconds: int64(T3402Backoff / time.Second),
			MobileReachable:   timerStatus(&ue.mobileReachableTimer),
			ImplicitDetach:    timerStatus(&ue.implicitDetachTimer),
			Paging:            timerStatus(&ue.pagingTimer),
		},
		LastActivity: UELastActivityExport{
			Timestamp: ue.lastSeenTime(),
		},
		PDNConnections: pdnConnectionExports(ue),
	}

	if conn := ue.active.Load(); conn != nil {
		export.Timers.NASGuard = timerStatus(&conn.nasGuard)

		rc := &RANConnectionExport{
			MMEUES1APID: uint32(conn.MMEUES1APID),
			ENBUES1APID: uint32(conn.ENBUES1APID),
		}

		if s := m.radios[conn.conn]; s != nil {
			rc.ENBName = s.name
			export.LastActivity.ENBNode = s.name
		}

		export.RANConnection = rc
	}

	return export
}

// pdnConnectionExports snapshots the UE's PDN connections, keyed by EPS bearer
// identity. The caller holds ue.mu.
func pdnConnectionExports(ue *UeContext) map[string]PDNConnectionExport {
	if len(ue.Pdns) == 0 {
		return nil
	}

	out := make(map[string]PDNConnectionExport, len(ue.Pdns))

	for ebi, p := range ue.Pdns {
		pc := PDNConnectionExport{
			Ebi:                    p.Ebi,
			Apn:                    p.Apn,
			PdnType:                p.PdnType,
			Qci:                    p.Qci,
			Arp:                    p.Arp,
			SessionAMBRUplinkBps:   p.SessAmbrULBps,
			SessionAMBRDownlinkBps: p.SessAmbrDLBps,
			Tunnel:                 tunnelExport(p),
		}

		if p.UeIP.IsValid() {
			pc.UeIPv4Address = p.UeIP.String()
		}

		if p.UeIPv6Prefix.IsValid() {
			pc.UeIPv6Prefix = p.UeIPv6Prefix.String()
		}

		out[strconv.FormatUint(uint64(ebi), 10)] = pc
	}

	return out
}

func tunnelExport(p *PdnConnection) *TunnelExport {
	t := &TunnelExport{
		SgwS1UTEID: p.SgwFTEID.TEID,
		EnbS1UTEID: p.EnbFTEID.TEID,
	}

	if p.SgwFTEID.Addr.IsValid() {
		t.SgwS1UAddress = p.SgwFTEID.Addr.String()
	}

	if p.EnbFTEID.Addr.IsValid() {
		t.EnbS1UAddress = p.EnbFTEID.Addr.String()
	}

	if *t == (TunnelExport{}) {
		return nil
	}

	return t
}
