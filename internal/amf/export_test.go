// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

func addTestUE(t *testing.T, amf *amfContext.AMF, imsi string, setup func(*amfContext.AmfUe)) *amfContext.AmfUe {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("invalid IMSI %q: %v", imsi, err)
	}

	ue := amfContext.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	setup(ue)

	if err := amf.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	t.Cleanup(func() {
		amf.RemoveUEBySupi(supi)
	})

	return ue
}

func exportAndMarshal(t *testing.T, amf *amfContext.AMF) []map[string]any {
	t.Helper()

	exports, err := amf.ExportUEs(context.Background())
	if err != nil {
		t.Fatalf("ExportUEs: %v", err)
	}

	b, err := json.Marshal(exports)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	return result
}

func jsonMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()

	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in JSON", key)
	}

	sub, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("key %q is not a JSON object, got %T", key, v)
	}

	return sub
}

func TestExportUEsEmpty(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	exports, err := amf.ExportUEs(context.Background())
	if err != nil {
		t.Fatalf("ExportUEs returned unexpected error: %v", err)
	}

	if exports == nil {
		t.Fatal("expected non-nil slice, got nil")
	}
}

func TestExportJSON_MinimalUE(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	addTestUE(t, amf, "001010000000001", func(ue *amfContext.AmfUe) {})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	identity := jsonMap(t, ueExport, "identity")
	if supi, ok := identity["supi"].(string); !ok || supi != "imsi-001010000000001" {
		t.Fatalf("expected identity.supi to be 'imsi-001010000000001', got %v", identity["supi"])
	}

	state := jsonMap(t, ueExport, "state")
	if gmmState, ok := state["gmm_state"].(string); !ok || gmmState != "Deregistered" {
		t.Fatalf("expected state.gmm_state to be 'Deregistered', got %v", state["gmm_state"])
	}

	if ongoingProc, ok := state["ongoing_procedure"].(string); !ok || ongoingProc != "Nothing" {
		t.Fatalf("expected state.ongoing_procedure to be 'Nothing', got %v", state["ongoing_procedure"])
	}

	if secCtx, ok := state["security_context_available"].(bool); !ok || secCtx != false {
		t.Fatalf("expected state.security_context_available to be false, got %v", state["security_context_available"])
	}

	if macFailed, ok := state["mac_failed"].(bool); !ok || macFailed != false {
		t.Fatalf("expected state.mac_failed to be false, got %v", state["mac_failed"])
	}

	if _, ok := ueExport["ran_connection"]; ok {
		t.Fatal("expected ran_connection to be absent")
	}

	subscription := jsonMap(t, ueExport, "subscription")
	if _, ok := subscription["allowed_nssai"]; ok {
		t.Fatal("expected subscription.allowed_nssai to be absent")
	}

	if _, ok := subscription["ambr"]; ok {
		t.Fatal("expected subscription.ambr to be absent")
	}

	security := jsonMap(t, ueExport, "security")
	if cipherAlg, ok := security["ciphering_algorithm"].(string); !ok || cipherAlg != "NEA0" {
		t.Fatalf("expected security.ciphering_algorithm to be 'NEA0', got %v", security["ciphering_algorithm"])
	}

	if integrityAlg, ok := security["integrity_algorithm"].(string); !ok || integrityAlg != "NIA0" {
		t.Fatalf("expected security.integrity_algorithm to be 'NIA0', got %v", security["integrity_algorithm"])
	}

	if _, ok := identity["pei"]; ok {
		t.Fatal("expected identity.pei to be absent")
	}

	location := jsonMap(t, ueExport, "location")
	if _, ok := location["timezone"]; ok {
		t.Fatal("expected location.timezone to be absent")
	}

	lastActivity := jsonMap(t, ueExport, "last_activity")
	if _, ok := lastActivity["radio_node"]; ok {
		t.Fatal("expected last_activity.radio_node to be absent")
	}

	pduSessions, ok := ueExport["pdu_sessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected pdu_sessions to be a map, got %T", ueExport["pdu_sessions"])
	}

	if len(pduSessions) != 0 {
		t.Fatalf("expected pdu_sessions to be empty, got %d entries", len(pduSessions))
	}

	timers := jsonMap(t, ueExport, "timers")
	if t3512, ok := timers["t3512_value_seconds"].(float64); !ok || t3512 != 0 {
		t.Fatalf("expected timers.t3512_value_seconds to be 0, got %v", timers["t3512_value_seconds"])
	}

	if t3502, ok := timers["t3502_value_seconds"].(float64); !ok || t3502 != 0 {
		t.Fatalf("expected timers.t3502_value_seconds to be 0, got %v", timers["t3502_value_seconds"])
	}

	timerNames := []string{"t3513_paging", "t3565_notification", "t3560_auth", "t3550_registration", "t3555_config_update", "t3522_deregistration", "mobile_reachable", "implicit_deregistration"}
	for _, timerName := range timerNames {
		timerObj := jsonMap(t, timers, timerName)
		if active, ok := timerObj["active"].(bool); !ok || active != false {
			t.Fatalf("expected timers.%s.active to be false, got %v", timerName, timerObj["active"])
		}
	}
}

func TestExportJSON_FullyPopulatedUE(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	now := time.Now()
	ue := addTestUE(t, amf, "001010000000002", func(ue *amfContext.AmfUe) {
		ue.Pei = "imei-123456789012345"
		ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
		ue.Suci = "suci-0-001-01-0000-0-0-0000000001"
		ue.State = amfContext.Registered
		ue.OnGoing = amfContext.OnGoingProcedureNothing
		ue.SecurityContextAvailable = true
		ue.CipheringAlg = security.AlgCiphering128NEA2
		ue.IntegrityAlg = security.AlgIntegrity128NIA2
		ue.NgKsi = models.NgKsi{Ksi: 1}
		ue.Location = models.UserLocation{
			NrLocation: &models.NrLocation{
				Tai:                      &models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
				Ncgi:                     &models.Ncgi{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, NrCellID: "000000001"},
				AgeOfLocationInformation: 5,
				UeLocationTimestamp:      &now,
			},
		}
		ue.Tai = models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}
		ue.TimeZone = "+09:00"
		ue.RegistrationArea = []models.Tai{
			{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
			{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000002"},
		}
		ue.AllowedNssai = &models.Snssai{Sst: 1, Sd: "000001"}
		ue.Ambr = &models.Ambr{Uplink: "1000000", Downlink: "2000000"}
		ue.SmContextList[5] = &amfContext.SmContext{
			Ref:    "imsi-001010000000002-5",
			Snssai: &models.Snssai{Sst: 1, Sd: "000001"},
		}
		radio := &amfContext.Radio{Name: "gNB-001", RanUEs: make(map[int64]*amfContext.RanUe), Log: zap.NewNop()}
		ue.RanUe = &amfContext.RanUe{
			RanUeNgapID: 42,
			AmfUeNgapID: 100,
			Tai:         models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
			Radio:       radio,
			Log:         zap.NewNop(),
		}
		ue.T3513 = amfContext.NewTimer(1*time.Hour, 3, func(_ int32) {}, func() {})
		ue.T3512Value = 3600 * time.Second
		ue.T3502Value = 720 * time.Second
		ue.LastSeenAt = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
		ue.LastSeenRadio = "gNB-001"
		ue.RegistrationType5GS = 1
		ue.IdentityTypeUsedForRegistration = 1
		ue.RetransmissionOfInitialNASMsg = true
		ue.AuthFailureCauseSynchFailureTimes = 2
	})

	t.Cleanup(func() {
		ue.T3513.Stop()
	})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	identity := jsonMap(t, ueExport, "identity")
	if supi, ok := identity["supi"].(string); !ok || supi != "imsi-001010000000002" {
		t.Fatalf("expected identity.supi to be 'imsi-001010000000002', got %v", identity["supi"])
	}

	if pei, ok := identity["pei"].(string); !ok || pei != "imei-123456789012345" {
		t.Fatalf("expected identity.pei to be 'imei-123456789012345', got %v", identity["pei"])
	}

	if suci, ok := identity["suci"].(string); !ok || suci != "suci-0-001-01-0000-0-0-0000000001" {
		t.Fatalf("expected identity.suci to be 'suci-0-001-01-0000-0-0-0000000001', got %v", identity["suci"])
	}

	plmnID := jsonMap(t, identity, "plmn_id")
	if mcc, ok := plmnID["Mcc"].(string); !ok || mcc != "001" {
		t.Fatalf("expected identity.plmn_id.Mcc to be '001', got %v", plmnID["Mcc"])
	}

	if mnc, ok := plmnID["Mnc"].(string); !ok || mnc != "01" {
		t.Fatalf("expected identity.plmn_id.Mnc to be '01', got %v", plmnID["Mnc"])
	}

	state := jsonMap(t, ueExport, "state")
	if gmmState, ok := state["gmm_state"].(string); !ok || gmmState != "Registered" {
		t.Fatalf("expected state.gmm_state to be 'Registered', got %v", state["gmm_state"])
	}

	if secCtx, ok := state["security_context_available"].(bool); !ok || secCtx != true {
		t.Fatalf("expected state.security_context_available to be true, got %v", state["security_context_available"])
	}

	if macFailed, ok := state["mac_failed"].(bool); !ok || macFailed != false {
		t.Fatalf("expected state.mac_failed to be false, got %v", state["mac_failed"])
	}

	if ongoingProc, ok := state["ongoing_procedure"].(string); !ok || ongoingProc != "Nothing" {
		t.Fatalf("expected state.ongoing_procedure to be 'Nothing', got %v", state["ongoing_procedure"])
	}

	security := jsonMap(t, ueExport, "security")
	if cipherAlg, ok := security["ciphering_algorithm"].(string); !ok || cipherAlg != "NEA2" {
		t.Fatalf("expected security.ciphering_algorithm to be 'NEA2', got %v", security["ciphering_algorithm"])
	}

	if integrityAlg, ok := security["integrity_algorithm"].(string); !ok || integrityAlg != "NIA2" {
		t.Fatalf("expected security.integrity_algorithm to be 'NIA2', got %v", security["integrity_algorithm"])
	}

	ngKsi := jsonMap(t, security, "ng_ksi")
	if ksi, ok := ngKsi["Ksi"].(float64); !ok || ksi != 1 {
		t.Fatalf("expected security.ng_ksi.Ksi to be 1, got %v", ngKsi["Ksi"])
	}

	location := jsonMap(t, ueExport, "location")
	if timezone, ok := location["timezone"].(string); !ok || timezone != "+09:00" {
		t.Fatalf("expected location.timezone to be '+09:00', got %v", location["timezone"])
	}

	registrationArea, ok := location["registration_area"].([]any)
	if !ok {
		t.Fatalf("expected location.registration_area to be a slice, got %T", location["registration_area"])
	}

	if len(registrationArea) != 2 {
		t.Fatalf("expected location.registration_area to have 2 entries, got %d", len(registrationArea))
	}

	currentLoc := jsonMap(t, location, "current")
	if _, ok := currentLoc["NrLocation"]; !ok {
		t.Fatal("expected location.current.NrLocation to be present")
	}

	subscription := jsonMap(t, ueExport, "subscription")

	allowedNssai := jsonMap(t, subscription, "allowed_nssai")
	if sst, ok := allowedNssai["Sst"].(float64); !ok || sst != 1 {
		t.Fatalf("expected subscription.allowed_nssai.Sst to be 1, got %v", allowedNssai["Sst"])
	}

	if sd, ok := allowedNssai["Sd"].(string); !ok || sd != "000001" {
		t.Fatalf("expected subscription.allowed_nssai.Sd to be '000001', got %v", allowedNssai["Sd"])
	}

	ambr := jsonMap(t, subscription, "ambr")
	if uplink, ok := ambr["Uplink"].(string); !ok || uplink != "1000000" {
		t.Fatalf("expected subscription.ambr.Uplink to be '1000000', got %v", ambr["Uplink"])
	}

	if downlink, ok := ambr["Downlink"].(string); !ok || downlink != "2000000" {
		t.Fatalf("expected subscription.ambr.Downlink to be '2000000', got %v", ambr["Downlink"])
	}

	ranConn := jsonMap(t, ueExport, "ran_connection")
	if ranUeNgapID, ok := ranConn["ran_ue_ngap_id"].(float64); !ok || ranUeNgapID != 42 {
		t.Fatalf("expected ran_connection.ran_ue_ngap_id to be 42, got %v", ranConn["ran_ue_ngap_id"])
	}

	if amfUeNgapID, ok := ranConn["amf_ue_ngap_id"].(float64); !ok || amfUeNgapID != 100 {
		t.Fatalf("expected ran_connection.amf_ue_ngap_id to be 100, got %v", ranConn["amf_ue_ngap_id"])
	}

	if radioName, ok := ranConn["radio_name"].(string); !ok || radioName != "gNB-001" {
		t.Fatalf("expected ran_connection.radio_name to be 'gNB-001', got %v", ranConn["radio_name"])
	}

	timers := jsonMap(t, ueExport, "timers")
	if t3512, ok := timers["t3512_value_seconds"].(float64); !ok || t3512 != 3600 {
		t.Fatalf("expected timers.t3512_value_seconds to be 3600, got %v", timers["t3512_value_seconds"])
	}

	if t3502, ok := timers["t3502_value_seconds"].(float64); !ok || t3502 != 720 {
		t.Fatalf("expected timers.t3502_value_seconds to be 720, got %v", timers["t3502_value_seconds"])
	}

	t3513 := jsonMap(t, timers, "t3513_paging")
	if active, ok := t3513["active"].(bool); !ok || active != true {
		t.Fatalf("expected timers.t3513_paging.active to be true, got %v", t3513["active"])
	}

	if maxRetries, ok := t3513["max_retries"].(float64); !ok || maxRetries != 3 {
		t.Fatalf("expected timers.t3513_paging.max_retries to be 3, got %v", t3513["max_retries"])
	}

	pduSessions, ok := ueExport["pdu_sessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected pdu_sessions to be a map, got %T", ueExport["pdu_sessions"])
	}

	pduSession, ok := pduSessions["imsi-001010000000002-5"].(map[string]any)
	if !ok {
		t.Fatalf("expected pdu_sessions['imsi-001010000000002-5'] to be a map, got %T", pduSessions["imsi-001010000000002-5"])
	}

	if ref, ok := pduSession["ref"].(string); !ok || ref != "imsi-001010000000002-5" {
		t.Fatalf("expected pdu_sessions entry ref to be 'imsi-001010000000002-5', got %v", pduSession["ref"])
	}

	if _, ok := pduSession["snssai"]; !ok {
		t.Fatal("expected pdu_sessions entry snssai to be present")
	}

	registration := jsonMap(t, ueExport, "registration")
	if regType, ok := registration["type"].(float64); !ok || regType != 1 {
		t.Fatalf("expected registration.type to be 1, got %v", registration["type"])
	}

	if identityType, ok := registration["identity_type_used"].(float64); !ok || identityType != 1 {
		t.Fatalf("expected registration.identity_type_used to be 1, got %v", registration["identity_type_used"])
	}

	if retransmission, ok := registration["retransmission"].(bool); !ok || retransmission != true {
		t.Fatalf("expected registration.retransmission to be true, got %v", registration["retransmission"])
	}

	if authFailureSyncTimes, ok := registration["auth_failure_sync_times"].(float64); !ok || authFailureSyncTimes != 2 {
		t.Fatalf("expected registration.auth_failure_sync_times to be 2, got %v", registration["auth_failure_sync_times"])
	}

	lastActivity := jsonMap(t, ueExport, "last_activity")
	if radioNode, ok := lastActivity["radio_node"].(string); !ok || radioNode != "gNB-001" {
		t.Fatalf("expected last_activity.radio_node to be 'gNB-001', got %v", lastActivity["radio_node"])
	}

	if _, ok := lastActivity["timestamp"]; !ok {
		t.Fatal("expected last_activity.timestamp to be present")
	}
}

func TestExportJSON_NilTimers(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	addTestUE(t, amf, "001010000000003", func(ue *amfContext.AmfUe) {})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	timers := jsonMap(t, ueExport, "timers")

	timerNames := []string{"t3513_paging", "t3565_notification", "t3560_auth", "t3550_registration", "t3555_config_update", "t3522_deregistration", "mobile_reachable", "implicit_deregistration"}
	for _, timerName := range timerNames {
		timerObj := jsonMap(t, timers, timerName)
		if active, ok := timerObj["active"].(bool); !ok || active != false {
			t.Fatalf("expected timers.%s.active to be false, got %v", timerName, timerObj["active"])
		}

		if _, ok := timerObj["expire_count"]; ok {
			t.Fatalf("expected timers.%s.expire_count to be absent", timerName)
		}

		if _, ok := timerObj["max_retries"]; ok {
			t.Fatalf("expected timers.%s.max_retries to be absent", timerName)
		}
	}
}

func TestExportJSON_NilRanUe(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	addTestUE(t, amf, "001010000000004", func(ue *amfContext.AmfUe) {
		ue.RanUe = nil
	})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	if _, ok := ueExport["ran_connection"]; ok {
		t.Fatal("expected ran_connection to be absent")
	}
}

func TestExportJSON_EmptySmContextList(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	addTestUE(t, amf, "001010000000005", func(ue *amfContext.AmfUe) {})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	pduSessions, ok := ueExport["pdu_sessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected pdu_sessions to be a map, got %T", ueExport["pdu_sessions"])
	}

	if len(pduSessions) != 0 {
		t.Fatalf("expected pdu_sessions to be empty, got %d entries", len(pduSessions))
	}
}

func TestExportJSON_NilLocationPointers(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	addTestUE(t, amf, "001010000000006", func(ue *amfContext.AmfUe) {})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	location := jsonMap(t, ueExport, "location")
	current := jsonMap(t, location, "current")

	if nrLoc := current["NrLocation"]; nrLoc != nil {
		t.Fatalf("expected location.current.NrLocation to be nil, got %v", nrLoc)
	}

	if eutraLoc := current["EutraLocation"]; eutraLoc != nil {
		t.Fatalf("expected location.current.EutraLocation to be nil, got %v", eutraLoc)
	}

	if n3gaLoc := current["N3gaLocation"]; n3gaLoc != nil {
		t.Fatalf("expected location.current.N3gaLocation to be nil, got %v", n3gaLoc)
	}
}

func TestExportJSON_PDUSessionNilSMFContext(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)
	addTestUE(t, amf, "001010000000007", func(ue *amfContext.AmfUe) {
		ue.SmContextList[1] = &amfContext.SmContext{
			Ref:                "nonexistent-ref",
			Snssai:             &models.Snssai{Sst: 1, Sd: "000001"},
			PduSessionInactive: true,
		}
	})

	result := exportAndMarshal(t, amf)

	if len(result) != 1 {
		t.Fatalf("expected 1 UE in result, got %d", len(result))
	}

	ueExport := result[0]

	pduSessions, ok := ueExport["pdu_sessions"].(map[string]any)
	if !ok {
		t.Fatalf("expected pdu_sessions to be a map, got %T", ueExport["pdu_sessions"])
	}

	pduSession, ok := pduSessions["nonexistent-ref"].(map[string]any)
	if !ok {
		t.Fatalf("expected pdu_sessions['nonexistent-ref'] to be a map, got %T", pduSessions["nonexistent-ref"])
	}

	if ref, ok := pduSession["ref"].(string); !ok || ref != "nonexistent-ref" {
		t.Fatalf("expected ref to be 'nonexistent-ref', got %v", pduSession["ref"])
	}

	if inactive, ok := pduSession["inactive"].(bool); !ok || inactive != true {
		t.Fatalf("expected inactive to be true, got %v", pduSession["inactive"])
	}

	if _, ok := pduSession["snssai"]; !ok {
		t.Fatal("expected snssai to be present")
	}

	if _, ok := pduSession["dnn"]; ok {
		t.Fatal("expected dnn to be absent")
	}

	if _, ok := pduSession["policy_data"]; ok {
		t.Fatal("expected policy_data to be absent")
	}

	if _, ok := pduSession["tunnel"]; ok {
		t.Fatal("expected tunnel to be absent")
	}

	if _, ok := pduSession["pfcp_local_seid"]; ok {
		t.Fatal("expected pfcp_local_seid to be absent")
	}

	if _, ok := pduSession["release_due_to_dup_id"]; ok {
		t.Fatal("expected release_due_to_dup_id to be absent")
	}
}
