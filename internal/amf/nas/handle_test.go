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
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

// setTestUESecurityCapability gives a UE the state a registered UE carries by the
// time an Initial Context Setup is built: a 5G security capability (TS 33.501)
// and a serving PLMN for the mobility restriction list.
func setTestUESecurityCapability(ue *amf.UeContext) {
	ue.SetUESecurityCapabilityForTest([]byte{0x00, 0x00})

	if ue.PlmnID.Mcc == "" {
		ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	}
}

type fakeDBInstance struct {
	Operator *db.Operator
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
	return &db.Profile{ID: id, Name: "TestProfile", Allow4G: true, Allow5G: true}, nil
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
	return &db.Policy{ID: "policy-1", Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1"}, nil
}

func (fdb *fakeDBInstance) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{
		{ID: "policy-1", Name: "TestPolicy", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"},
		{ID: "policy-2", Name: "TestPolicy2", ProfileID: "profile-1", SliceID: "slice-2", DataNetworkID: "dn-1"},
	}, nil
}

func (fdb *fakeDBInstance) NodeID() int { return 0 }

type NGDLNasTransport struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	NasPdu      []byte
}

type NGPDUSessionResourceSetupRequest struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	AmbrUL      string
	AmbrDL      string
	NasPdu      []byte
	SuList      ngapType.PDUSessionResourceSetupListSUReq
}

type NGInitialContextSetupRequest struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	AmbrUL      string
	AmbrDL      string
	NasPdu      []byte
	CtxList     ngapType.PDUSessionResourceSetupListCxtReq
}

type NGUEContextReleaseCommand struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
}

type NGPDUSessionResourceReleaseCommand struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	NasPdu      []byte
	List        ngapType.PDUSessionResourceToReleaseListRelCmd
}

type fakeNGAPSender struct {
	SentDownlinkNASTransport             []*NGDLNasTransport
	SentPDUSessionResourceSetupRequest   []*NGPDUSessionResourceSetupRequest
	SentInitialContextSetupRequest       []*NGInitialContextSetupRequest
	SentUEContextReleaseCommand          []*NGUEContextReleaseCommand
	SentPDUSessionResourceReleaseCommand []*NGPDUSessionResourceReleaseCommand
}

// WriteMsg decodes the sent NGAP PDU and records the NAS PDU it carries in the
// bucket matching its procedure, so NAS tests assert on the downlink message the
// same way the sender's typed buckets did.
func (fng *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Decoder(b)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: decode NGAP PDU: %v", err))
	}

	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		return len(b), nil
	}

	m := pdu.InitiatingMessage

	switch m.ProcedureCode.Value {
	case ngapType.ProcedureCodeDownlinkNASTransport:
		msg := &NGDLNasTransport{}

		for _, ie := range m.Value.DownlinkNASTransport.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				msg.AmfUeNGAPID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				msg.RanUeNGAPID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDNASPDU:
				msg.NasPdu = ie.Value.NASPDU.Value
			}
		}

		fng.SentDownlinkNASTransport = append(fng.SentDownlinkNASTransport, msg)

	case ngapType.ProcedureCodeInitialContextSetup:
		msg := &NGInitialContextSetupRequest{}

		for _, ie := range m.Value.InitialContextSetupRequest.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				msg.AmfUeNGAPID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				msg.RanUeNGAPID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDNASPDU:
				msg.NasPdu = ie.Value.NASPDU.Value
			case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
				msg.CtxList = *ie.Value.PDUSessionResourceSetupListCxtReq
			}
		}

		fng.SentInitialContextSetupRequest = append(fng.SentInitialContextSetupRequest, msg)

	case ngapType.ProcedureCodePDUSessionResourceSetup:
		msg := &NGPDUSessionResourceSetupRequest{}

		for _, ie := range m.Value.PDUSessionResourceSetupRequest.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				msg.AmfUeNGAPID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				msg.RanUeNGAPID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDNASPDU:
				msg.NasPdu = ie.Value.NASPDU.Value
			case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
				msg.SuList = *ie.Value.PDUSessionResourceSetupListSUReq
			}
		}

		fng.SentPDUSessionResourceSetupRequest = append(fng.SentPDUSessionResourceSetupRequest, msg)

	case ngapType.ProcedureCodePDUSessionResourceRelease:
		msg := &NGPDUSessionResourceReleaseCommand{}

		for _, ie := range m.Value.PDUSessionResourceReleaseCommand.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				msg.AmfUeNGAPID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				msg.RanUeNGAPID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDNASPDU:
				msg.NasPdu = ie.Value.NASPDU.Value
			case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
				msg.List = *ie.Value.PDUSessionResourceToReleaseListRelCmd
			}
		}

		fng.SentPDUSessionResourceReleaseCommand = append(fng.SentPDUSessionResourceReleaseCommand, msg)

	case ngapType.ProcedureCodeUEContextRelease:
		msg := &NGUEContextReleaseCommand{}

		for _, ie := range m.Value.UEContextReleaseCommand.ProtocolIEs.List {
			if ie.Id.Value == ngapType.ProtocolIEIDUENGAPIDs && ie.Value.UENGAPIDs.UENGAPIDPair != nil {
				msg.AmfUeNGAPID = ie.Value.UENGAPIDs.UENGAPIDPair.AMFUENGAPID.Value
				msg.RanUeNGAPID = ie.Value.UENGAPIDs.UENGAPIDPair.RANUENGAPID.Value
			}
		}

		fng.SentUEContextReleaseCommand = append(fng.SentUEContextReleaseCommand, msg)
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
	Error             error
	ReleasedSmContext []string

	// ActivateSmContext fields
	ActivateSmContextResponse []byte
	ActivateSmContextError    error
	ActivateSmContextCalls    []SmfActivateSmContextCall

	// ReleaseSmContext fields
	ReleaseSmContextError error
	ReleaseSmContextCalls []SmfReleaseSmContextCall

	// UpdateSmContextN1Msg fields
	UpdateN1MsgResponse *smf.UpdateResult
	UpdateN1MsgError    error
	UpdateN1MsgCalls    []SmfUpdateN1MsgCall

	// CreateSmContext fields
	CreateSmContextRef     string
	CreateSmContextErrResp []byte
	CreateSmContextError   error
	CreateSmContextCalls   []SmfCreateSmContextCall

	// UpdateSmContextCauseDuplicatePDUSessionID fields
	DuplicatePDUResponse []byte
	DuplicatePDUError    error
	DuplicatePDUCalls    []SmfDuplicatePDUCall
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

func (s *fakeSmf) UpdateSmContextHandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error) {
	s.UpdateN1MsgCalls = append(s.UpdateN1MsgCalls, SmfUpdateN1MsgCall{
		SmContextRef: smContextRef,
		N1Msg:        n1Msg,
	})

	return s.UpdateN1MsgResponse, s.UpdateN1MsgError
}

func (s *fakeSmf) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	s.CreateSmContextCalls = append(s.CreateSmContextCalls, SmfCreateSmContextCall{
		Supi:         supi,
		PduSessionID: pduSessionID,
		Dnn:          dnn,
		Snssai:       snssai,
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

func (s *fakeSmf) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, _ string) error {
	return s.Error
}

func (s *fakeSmf) UpdateSmContextN2HandoverPreparing(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *fakeSmf) UpdateSmContextN2HandoverPrepared(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, s.Error
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
