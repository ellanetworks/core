// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func setTestUESecurityCapability(ue *amf.UeContext) {
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})

	if ue.PlmnID.Mcc == "" {
		ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	}
}

type fakeDBInstance struct {
	Operator  *db.Operator
	BarFrom5G bool
}

func (fdb *fakeDBInstance) GetOperator(ctx context.Context) (*db.Operator, error) {
	if fdb.Operator == nil {
		return nil, fmt.Errorf("could not get operator")
	}

	return fdb.Operator, nil
}

func (fdb *fakeDBInstance) GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error) {
	return &db.DataNetwork{
		ID:   id,
		Name: "TestDataNetwork",
	}, nil
}

func (fdb *fakeDBInstance) GetNetworkSliceByID(_ context.Context, id string) (*db.NetworkSlice, error) {
	sd1 := "010203"
	sd2 := "aabbcc"
	slices := map[string]*db.NetworkSlice{
		"slice-1": {ID: "slice-1", Name: "default", Sst: 1, Sd: &sd1},
		"slice-2": {ID: "slice-2", Name: "secondary", Sst: 1, Sd: &sd2},
	}

	s, ok := slices[id]
	if !ok {
		return nil, fmt.Errorf("slice %s not found", id)
	}

	return s, nil
}

func (fdb *fakeDBInstance) ListNetworkSlicesByIDs(_ context.Context, ids []string) ([]db.NetworkSlice, error) {
	sd1 := "010203"
	sd2 := "aabbcc"
	slices := map[string]db.NetworkSlice{
		"slice-1": {ID: "slice-1", Name: "default", Sst: 1, Sd: &sd1},
		"slice-2": {ID: "slice-2", Name: "secondary", Sst: 1, Sd: &sd2},
	}

	var out []db.NetworkSlice

	for _, id := range ids {
		if s, ok := slices[id]; ok {
			out = append(out, s)
		}
	}

	return out, nil
}

func (fdb *fakeDBInstance) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{
		Imsi: imsi,
	}, nil
}

func (fdb *fakeDBInstance) GetProfileByID(ctx context.Context, id string) (*db.Profile, error) {
	return &db.Profile{ID: id, Name: "TestProfile", Allow4G: true, Allow5G: !fdb.BarFrom5G, UeAmbrDownlink: "200 Mbps", UeAmbrUplink: "100 Mbps"}, nil
}

func (fdb *fakeDBInstance) ListAllNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	sd1 := "010203"
	sd2 := "aabbcc"

	return []db.NetworkSlice{
		{ID: "slice-1", Name: "default", Sst: 1, Sd: &sd1},
		{ID: "slice-2", Name: "secondary", Sst: 1, Sd: &sd2},
	}, nil
}

func (fdb *fakeDBInstance) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*db.Policy, error) {
	return &db.Policy{ID: "policy-1", Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1", SessionAmbrDownlink: "200 Mbps", SessionAmbrUplink: "100 Mbps"}, nil
}

func (fdb *fakeDBInstance) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{
		{ID: "policy-1", Name: "TestPolicy", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"},
		{ID: "policy-2", Name: "TestPolicy2", ProfileID: "profile-1", SliceID: "slice-2", DataNetworkID: "dn-1"},
	}, nil
}

func (fdb *fakeDBInstance) NodeID() int { return 0 }

type fakeNGAPSender struct {
	SentDownlinkNASTransport               []*ngap.DownlinkNASTransport
	SentPDUSessionResourceSetupRequest     []*ngap.PDUSessionResourceSetupRequest
	SentInitialContextSetupRequest         []*ngap.InitialContextSetupRequest
	SentUEContextReleaseCommand            []*ngap.UEContextReleaseCommand
	SentPDUSessionResourceReleaseCommand   []*ngap.PDUSessionResourceReleaseCommand
	SentDownlinkUEAssociatedNRPPaTransport []*ngap.DownlinkUEAssociatedNRPPaTransport
}

func capture[M any](bucket *[]*M, parse func([]byte) (*M, error), value []byte, name string) {
	m, err := parse(value)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: parse %s: %v", name, err))
	}

	*bucket = append(*bucket, m)
}

func (fng *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: unmarshal NGAP PDU: %v", err))
	}

	m, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		return len(b), nil
	}

	switch m.ProcedureCode {
	case ngap.ProcDownlinkNASTransport:
		capture(&fng.SentDownlinkNASTransport, ngap.ParseDownlinkNASTransport, m.Value, "Downlink NAS Transport")
	case ngap.ProcInitialContextSetup:
		capture(&fng.SentInitialContextSetupRequest, ngap.ParseInitialContextSetupRequest, m.Value, "Initial Context Setup Request")
	case ngap.ProcPDUSessionResourceSetup:
		capture(&fng.SentPDUSessionResourceSetupRequest, ngap.ParsePDUSessionResourceSetupRequest, m.Value, "PDU Session Resource Setup Request")
	case ngap.ProcPDUSessionResourceRelease:
		capture(&fng.SentPDUSessionResourceReleaseCommand, ngap.ParsePDUSessionResourceReleaseCommand, m.Value, "PDU Session Resource Release Command")
	case ngap.ProcUEContextRelease:
		capture(&fng.SentUEContextReleaseCommand, ngap.ParseUEContextReleaseCommand, m.Value, "UE Context Release Command")
	case ngap.ProcDownlinkUEAssociatedNRPPaTransport:
		capture(&fng.SentDownlinkUEAssociatedNRPPaTransport, ngap.ParseDownlinkUEAssociatedNRPPaTransport, m.Value, "Downlink UE-Associated NRPPa Transport")
	}

	return len(b), nil
}

type fakeAusf struct {
	Supi    etsi.SUPI
	Kseaf   []byte
	Error   error
	AvKgAka *ausf.AuthResult
}

func (a *fakeAusf) Authenticate(ctx context.Context, suci string, plmn models.PlmnID, resync *ausf.ResyncInfo) (*ausf.AuthResult, error) {
	if a.Error != nil {
		return nil, a.Error
	}

	return a.AvKgAka, nil
}

func (a *fakeAusf) Confirm(ctx context.Context, resStar string, suci string) (etsi.SUPI, []byte, error) {
	if a.Error != nil {
		return etsi.InvalidSUPI, nil, a.Error
	}

	return a.Supi, a.Kseaf, nil
}

func mustSUPIFromPrefixed(s string) etsi.SUPI { //nolint:unparam
	supi, err := etsi.NewSUPIFromPrefixed(s)
	if err != nil {
		panic("mustSUPIFromPrefixed: " + err.Error())
	}

	return supi
}

type SmfActivateSmContextCall struct {
	SmContextRef string
}

type SmfReleaseSmContextCall struct {
	SmContextRef string
}

type fakeSmf struct {
	Error                     error
	ReleasedSmContext         []string
	IdleTransfers             []idleTransfer
	IdleTransferErr           error
	ActivateSmContextResponse []byte
	ActivateSmContextError    error
	ActivateSmContextCalls    []SmfActivateSmContextCall
	ReleaseSmContextError     error
	ReleaseSmContextCalls     []SmfReleaseSmContextCall
	UpdateN1MsgResponse       *smf.UpdateResult
	UpdateN1MsgError          error
	UpdateN1MsgCalls          []SmfUpdateN1MsgCall
	CreateSmContextRef        string
	CreateSmContextErrResp    []byte
	CreateSmContextError      error
	CreateSmContextCalls      []SmfCreateSmContextCall
	DuplicatePDUResponse      []byte
	DuplicatePDUError         error
	DuplicatePDUCalls         []SmfDuplicatePDUCall
}

type SmfUpdateN1MsgCall struct {
	SmContextRef string
	N1Msg        []byte
}

type SmfCreateSmContextCall struct {
	Supi         etsi.SUPI
	PduSessionID uint8
	Dnn          string
	Snssai       *models.Snssai
	RequestType  fgs.RequestType
	N1Msg        []byte
}

type SmfDuplicatePDUCall struct {
	SmContextRef string
}

func (s *fakeSmf) ActivateSmContext(_ context.Context, smContextRef string) ([]byte, error) {
	s.ActivateSmContextCalls = append(s.ActivateSmContextCalls, SmfActivateSmContextCall{
		SmContextRef: smContextRef,
	})

	if s.ActivateSmContextError != nil {
		return nil, s.ActivateSmContextError
	}

	if s.Error != nil {
		return nil, s.Error
	}

	resp := s.ActivateSmContextResponse
	if resp == nil {
		resp = []byte{}
	}

	return resp, nil
}

func (s *fakeSmf) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	s.ReleaseSmContextCalls = append(s.ReleaseSmContextCalls, SmfReleaseSmContextCall{
		SmContextRef: smContextRef,
	})

	if s.ReleaseSmContextError != nil {
		return s.ReleaseSmContextError
	}

	if s.Error != nil {
		return s.Error
	}

	s.ReleasedSmContext = append(s.ReleasedSmContext, smContextRef)

	return nil
}

func (s *fakeSmf) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *fakeSmf) UpdateSmContextN2ModifyIndication(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *fakeSmf) UpdateSmContextXnHandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN2HandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error) {
	s.UpdateN1MsgCalls = append(s.UpdateN1MsgCalls, SmfUpdateN1MsgCall{
		SmContextRef: smContextRef,
		N1Msg:        n1Msg,
	})

	return s.UpdateN1MsgResponse, s.UpdateN1MsgError
}

func (s *fakeSmf) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, requestType fgs.RequestType, n1Msg []byte, _ uint8) (string, []byte, error) {
	s.CreateSmContextCalls = append(s.CreateSmContextCalls, SmfCreateSmContextCall{
		Supi:         supi,
		PduSessionID: pduSessionID,
		Dnn:          dnn,
		Snssai:       snssai,
		RequestType:  requestType,
		N1Msg:        n1Msg,
	})

	return s.CreateSmContextRef, s.CreateSmContextErrResp, s.CreateSmContextError
}

func (s *fakeSmf) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	s.DuplicatePDUCalls = append(s.DuplicatePDUCalls, SmfDuplicatePDUCall{
		SmContextRef: smContextRef,
	})

	return s.DuplicatePDUResponse, s.DuplicatePDUError
}

func (s *fakeSmf) DeactivateSmContext(_ context.Context, _ string) error {
	return s.Error
}

func (s *fakeSmf) HandlePagingFailure(_ context.Context, _ etsi.SUPI, _ uint8) error {
	return s.Error
}

func (s *fakeSmf) ClearPagingSuppression(_ context.Context, _ etsi.SUPI, _ uint8) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN2InfoPduResSetupRsp(_ context.Context, _ string, _ []byte) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN2InfoPduResSetupFail(_ context.Context, _ string, _ []byte) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, _ string) (bool, error) {
	return false, s.Error
}

func (s *fakeSmf) PrepareSmContextFromEPS(_ context.Context, _ etsi.SUPI, _, _ uint8, _ string, _ *models.Snssai) (string, []byte, error) {
	return "", nil, nil
}

type idleTransfer struct {
	Supi              etsi.SUPI
	PDUSessionID      uint8
	EPSBearerIdentity uint8
	Dnn               string
	Snssai            *models.Snssai
}

func (s *fakeSmf) TransferIdleTo5GS(_ context.Context, supi etsi.SUPI, pduSessionID, epsBearerIdentity uint8, dnn string, snssai *models.Snssai) (string, error) {
	s.IdleTransfers = append(s.IdleTransfers, idleTransfer{
		Supi:              supi,
		PDUSessionID:      pduSessionID,
		EPSBearerIdentity: epsBearerIdentity,
		Dnn:               dnn,
		Snssai:            snssai,
	})

	if s.IdleTransferErr != nil {
		return "", s.IdleTransferErr
	}

	return fmt.Sprintf("idle-ref-%d", pduSessionID), nil
}

func (s *fakeSmf) UpdateSmContextN2HandoverPreparing(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *fakeSmf) UpdateSmContextN2HandoverPrepared(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *fakeSmf) UpdateSmContextN2HandoverCanceled(_ context.Context, _ string) error {
	return nil
}

func (s *fakeSmf) UpdateSmContextN2HandoverComplete(_ context.Context, _ string) error {
	return s.Error
}

func (s *fakeSmf) GetSession(_ string) *smf.SMContext { return nil }

func (s *fakeSmf) SessionsByDNN(_ string) []*smf.SMContext { return nil }

func (s *fakeSmf) SessionCount() int { return 0 }

func (s *fakeSmf) ReconcileSmContext(_ context.Context, _ *models.SessionReconcileRequest) error {
	return s.Error
}

func (s *fakeSmf) GetSessionPolicy(_ context.Context, _ etsi.SUPI, _ *models.Snssai, _ string) (*smf.Policy, error) {
	return nil, nil
}

func mustTestGuti(mcc string, mnc string, amfid string, tmsi uint32) etsi.GUTI5G {
	t, err := etsi.NewTMSI(tmsi)
	if err != nil {
		panic("invalid tmsi")
	}

	guti, err := etsi.NewGUTI5G(mcc, mnc, amfid, t)
	if err != nil {
		panic("invalid guti")
	}

	return guti
}
