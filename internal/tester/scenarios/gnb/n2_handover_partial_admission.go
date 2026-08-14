// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/nas/fgs"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

const (
	partialAdmissionIMSI  = "001017271246597"
	partialAdmissionDNN   = "handover-enterprise"
	partialAdmissionSD    = "204070"
	partialAdmissionSST   = 1
	partialKeptSession    = uint8(1)
	partialDroppedSession = uint8(2)
	partialTargetRANID    = int64(400)
	partialTargetTEID     = uint32(9400)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/n2_handover_partial_admission",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2HandoverPartialAdmission,
		Fixture:   fixtureN2HandoverPartialAdmission,
	})
}

func fixtureN2HandoverPartialAdmission(env scenarios.Env) scenarios.FixtureSpec {
	pool := "10.47.0.0/16"
	dns := scenarios.DefaultDNS

	if env.IPFamily() == scenarios.IPv6Only {
		pool = "fd47::/48"
		dns = scenarios.DefaultDNSv6
	}

	return scenarios.FixtureSpec{
		Slices: []scenarios.SliceSpec{
			{Name: "handover-slice", SST: partialAdmissionSST, SD: partialAdmissionSD},
		},
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: partialAdmissionDNN, IPv4Pool: pool, DNS: dns, MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "handover-partial-enterprise",
				ProfileName:         scenarios.DefaultProfileName,
				SliceName:           "handover-slice",
				DataNetworkName:     partialAdmissionDNN,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(partialAdmissionIMSI, ""),
		},
	}
}

func runN2HandoverPartialAdmission(_ context.Context, env scenarios.Env, _ any) error {
	sourceGNB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer sourceGNB.Close()

	targetGNB, err := startXnTargetGNB(env, env.FirstGNB().N2Address, "")
	if err != nil {
		return err
	}

	defer targetGNB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(sourceGNB, partialAdmissionIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: partialKeptSession,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	amfUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID)

	if err := newUE.SendPDUSessionEstablishmentRequest(amfUENGAPID, ranUENGAPID, partialDroppedSession, partialAdmissionDNN,
		models.Snssai{Sst: int32(partialAdmissionSST), Sd: partialAdmissionSD}); err != nil {
		return fmt.Errorf("establish the second PDU session: %w", err)
	}

	if _, err := newUE.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), 5*time.Second); err != nil {
		return fmt.Errorf("the second PDU session was not accepted: %w", err)
	}

	for _, id := range []uint8{partialKeptSession, partialDroppedSession} {
		if _, err := sourceGNB.WaitForPDUSession(ranUENGAPID, int64(id), 5*time.Second); err != nil {
			return fmt.Errorf("source gNB: wait PDU session %d: %w", id, err)
		}
	}

	if err := sourceGNB.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  amfUENGAPID,
		RANUENGAPID:  ranUENGAPID,
		HandoverType: ngaplib.HandoverTypeIntra5GS,
		TargetGnbID:  handoverTargetGnbID,
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(partialKeptSession)},
			{PDUSessionID: int64(partialDroppedSession)},
		},
	}); err != nil {
		return fmt.Errorf("send HandoverRequired: %w", err)
	}

	req, err := targetGNB.WaitForHandoverRequest(5 * time.Second)
	if err != nil {
		return fmt.Errorf("the target gNB got no HandoverRequest: %w", err)
	}

	if len(req.PDUSessionResourceSetupListHOReq) != 2 {
		return fmt.Errorf("the HandoverRequest carried %d PDU sessions, want 2", len(req.PDUSessionResourceSetupListHOReq))
	}

	refusal := ngaplib.Cause{Group: ngaplib.CauseGroupRadioNetwork, Value: ngaplib.CauseRadioNetworkRadioResourcesNotAvailable}

	if err := targetGNB.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: int64(req.AMFUENGAPID),
		RANUENGAPID: partialTargetRANID,
		PDUSessions: []gnb.HandoverAdmittedPDUSession{
			{
				PDUSessionID: int64(partialKeptSession),
				DLTeid:       partialTargetTEID,
				DLIP:         netip.MustParseAddr(env.FirstGNB().N3Address),
			},
		},
		FailedPDUSessions: []gnb.HandoverFailedPDUSession{
			{PDUSessionID: int64(partialDroppedSession), Cause: refusal},
		},
		TargetToSourceTransparentContainer: n2HandoverRRCContainer,
	}); err != nil {
		return fmt.Errorf("admit one session at the target gNB: %w", err)
	}

	frame, err := sourceGNB.WaitForMessage(gnb.Successful, ngaplib.ProcHandoverPreparation, 5*time.Second)
	if err != nil {
		return fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	cmd, err := ngaplib.ParseHandoverCommand(frame.Value)
	if err != nil {
		return fmt.Errorf("parse HandoverCommand: %w", err)
	}

	if len(cmd.PDUSessionResourceHandoverList) != 1 {
		return fmt.Errorf("the HandoverCommand handed over %d PDU sessions, want only the admitted one", len(cmd.PDUSessionResourceHandoverList))
	}

	if got := cmd.PDUSessionResourceHandoverList[0].PDUSessionID; int(got) != int(partialKeptSession) {
		return fmt.Errorf("the HandoverCommand handed over PDU session %d, want the admitted %d", got, partialKeptSession)
	}

	if len(cmd.PDUSessionResourceToReleaseList) != 1 {
		return fmt.Errorf("the HandoverCommand told the source to release %d PDU sessions, want the one the target refused", len(cmd.PDUSessionResourceToReleaseList))
	}

	if got := cmd.PDUSessionResourceToReleaseList[0].PDUSessionID; int(got) != int(partialDroppedSession) {
		return fmt.Errorf("the HandoverCommand released PDU session %d, want the refused %d", got, partialDroppedSession)
	}

	if err := relayRANStatusTransfer(sourceGNB, targetGNB, amfUENGAPID, ranUENGAPID, partialTargetRANID); err != nil {
		return err
	}

	if err := targetGNB.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: int64(req.AMFUENGAPID),
		RANUENGAPID: partialTargetRANID,
	}); err != nil {
		return fmt.Errorf("send HandoverNotify: %w", err)
	}

	if _, err := sourceGNB.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 5*time.Second); err != nil {
		return errors.New("the source gNB was not released after the UE moved")
	}

	return nil
}
