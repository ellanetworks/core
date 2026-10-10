// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/nas/fgs"
	ngaplib "github.com/ellanetworks/core/ngap"
)

const (
	imsHandoverTargetRANID = int64(300)
	imsHandoverTargetTEID  = uint32(9300)
	pagingTimeout          = 10 * time.Second
)

var (
	imsHoldCaller       = scenarios.IMSSubscriber("001018400000005", "+15558400005")
	imsHoldCallee       = scenarios.IMSSubscriber("001018400000006", "+15558400006")
	imsSharingCaller    = scenarios.IMSSubscriber("001018400000007", "+15558400007")
	imsSharingCallee    = scenarios.IMSSubscriber("001018400000008", "+15558400008")
	imsModRejectCaller  = scenarios.IMSSubscriber("001018400000009", "+15558400009")
	imsModRejectCallee  = scenarios.IMSSubscriber("001018400000010", "+15558400010")
	imsReleasedCaller   = scenarios.IMSSubscriber("001018400000011", "+15558400011")
	imsReleasedCallee   = scenarios.IMSSubscriber("001018400000012", "+15558400012")
	imsRadioLostCaller  = scenarios.IMSSubscriber("001018400000013", "+15558400013")
	imsRadioLostCallee  = scenarios.IMSSubscriber("001018400000014", "+15558400014")
	imsIdleCaller       = scenarios.IMSSubscriber("001018400000015", "+15558400015")
	imsIdleCallee       = scenarios.IMSSubscriber("001018400000016", "+15558400016")
	imsXnHandoverCaller = scenarios.IMSSubscriber("001018400000017", "+15558400017")
	imsXnHandoverCallee = scenarios.IMSSubscriber("001018400000018", "+15558400018")
	imsN2HandoverCaller = scenarios.IMSSubscriber("001018400000019", "+15558400019")
	imsN2HandoverCallee = scenarios.IMSSubscriber("001018400000020", "+15558400020")
)

func init() {
	registerIMS("ims/5g_call_hold", imsHoldCaller, runIMSCallHold, imsHoldCallee)
	registerIMS("ims/5g_two_calls", imsSharingCaller, runIMSTwoCalls, imsSharingCallee)
	registerIMS("ims/5g_two_calls_modification_rejected", imsModRejectCaller, runIMSModificationRejected, imsModRejectCallee)
	registerIMS("ims/5g_call_flow_released", imsReleasedCaller, func(ctx context.Context, env scenarios.Env) error {
		return runIMSCallLoss(ctx, env, imsReleasedCaller, imsReleasedCallee, releaseFlowAtGNB)
	}, imsReleasedCallee)
	registerIMS("ims/5g_call_radio_lost", imsRadioLostCaller, func(ctx context.Context, env scenarios.Env) error {
		return runIMSCallLoss(ctx, env, imsRadioLostCaller, imsRadioLostCallee, loseRadioConnection)
	}, imsRadioLostCallee)
	registerIMS("ims/5g_call_idle", imsIdleCaller, runIMSCallIdle, imsIdleCallee)
	registerIMS("ims/5g_call_xn_handover", imsXnHandoverCaller, runIMSCallXnHandover, imsXnHandoverCallee)
	registerIMS("ims/5g_call_n2_handover", imsN2HandoverCaller, runIMSCallN2Handover, imsN2HandoverCallee)
}

type imsCallSetup struct {
	g         *gnb.GnodeB
	endpoints []scenarios.IMSEndpoint
	ues       []imsCallUE
	legs      []voiceLeg
	close     func()
}

func setUpIMSCall(env scenarios.Env, caller, callee scenarios.SubscriberSpec) (*imsCallSetup, error) {
	g, err := startGNB(env)
	if err != nil {
		return nil, err
	}

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, g, caller, callee)
	if err != nil {
		g.Close()
		return nil, err
	}

	return &imsCallSetup{
		g:         g,
		endpoints: endpoints,
		ues:       ues,
		legs:      callLegs(ues),
		close: func() {
			cleanup()
			g.Close()
		},
	}, nil
}

func flowSignallingCounts(ues []imsCallUE) []int64 {
	counts := make([]int64, 0, 2*len(ues))
	for _, u := range ues {
		counts = append(counts, int64(u.leg.gnb.ModifyRequestCount(u.leg.ranUEID)), u.ue.ModificationCommandCount())
	}

	return counts
}

func requireNoFlowSignalling(ues []imsCallUE, before []int64) error {
	if now := flowSignallingCounts(ues); !slices.Equal(now, before) {
		return fmt.Errorf("N2 Modify Requests and N1 Modification Commands per UE went from %v to %v, want the gates applied in the UPF only (TS 29.513 §5.2.2.2)", before, now)
	}

	return nil
}

func runIMSCallHold(ctx context.Context, env scenarios.Env) error {
	c, err := setUpIMSCall(env, imsHoldCaller, imsHoldCallee)
	if err != nil {
		return err
	}

	defer c.close()

	var (
		qfis     []uint8
		modifies []int64
	)

	steps := scenarios.IMSHoldSteps{
		Up: func(ctx context.Context, m scenarios.IMSMedia) error {
			var err error

			if qfis, err = awaitVoiceQFIs(ctx, c.legs); err != nil {
				return err
			}

			modifies = flowSignallingCounts(c.ues)

			return sendVoiceBothWays(ctx, c.legs, qfis, m)
		},
		Held: func(ctx context.Context, m scenarios.IMSMedia) error {
			if err := requireVoiceGated(ctx, c.legs[1], qfis[1], c.legs[0], m.Callee, m.Caller); err != nil {
				return fmt.Errorf("callee to held caller (TS 29.214 §5.3.11): %w", err)
			}

			if err := sendVoiceOnQFI(ctx, c.legs[0], qfis[0], c.legs[1], qfis[1], m.Caller, m.Callee); err != nil {
				return fmt.Errorf("held caller to callee: %w", err)
			}

			rtcp := scenarios.IMSMedia{Caller: scenarios.RTCPEndpoint(m.Caller), Callee: scenarios.RTCPEndpoint(m.Callee)}

			if err := sendVoiceBothWays(ctx, c.legs, qfis, rtcp); err != nil {
				return fmt.Errorf("RTCP on hold (TS 29.214 §4.4.3): %w", err)
			}

			return requireNoFlowSignalling(c.ues, modifies)
		},
		Resumed: func(ctx context.Context, m scenarios.IMSMedia) error {
			if err := sendVoiceBothWays(ctx, c.legs, qfis, m); err != nil {
				return err
			}

			return requireNoFlowSignalling(c.ues, modifies)
		},
	}

	if err := scenarios.RequireIMSCallHeld(ctx, c.endpoints[0], c.endpoints[1], scenarios.IMSTransports[0], steps); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, c.legs)
}

func requireSameVoiceQFIs(ctx context.Context, legs []voiceLeg, want []uint8) error {
	got, err := awaitVoiceQFIs(ctx, legs)
	if err != nil {
		return err
	}

	if !slices.Equal(got, want) {
		return fmt.Errorf("the voice QoS flows moved from QFIs %v to %v, want the remaining call kept on its flow", want, got)
	}

	return nil
}

func runIMSTwoCalls(ctx context.Context, env scenarios.Env) error {
	c, err := setUpIMSCall(env, imsSharingCaller, imsSharingCallee)
	if err != nil {
		return err
	}

	defer c.close()

	var qfis []uint8

	steps := scenarios.IMSTwoCallSteps{
		BothUp: func(ctx context.Context, first, second scenarios.IMSMedia) error {
			var err error

			if qfis, err = awaitVoiceQFIs(ctx, c.legs); err != nil {
				return fmt.Errorf("want both calls on one voice QoS flow per UE (TS 23.501 §5.7.1.1): %w", err)
			}

			if err := sendVoiceBothWays(ctx, c.legs, qfis, first); err != nil {
				return fmt.Errorf("first call: %w", err)
			}

			if err := sendVoiceBothWays(ctx, c.legs, qfis, second); err != nil {
				return fmt.Errorf("second call: %w", err)
			}

			return nil
		},
		FirstEnded: func(ctx context.Context, second scenarios.IMSMedia) error {
			if err := requireSameVoiceQFIs(ctx, c.legs, qfis); err != nil {
				return err
			}

			return sendVoiceBothWays(ctx, c.legs, qfis, second)
		},
	}

	if err := scenarios.RequireIMSTwoCalls(ctx, c.endpoints[0], c.endpoints[1], scenarios.IMSTransports[0], steps); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, c.legs)
}

func runIMSModificationRejected(ctx context.Context, env scenarios.Env) error {
	c, err := setUpIMSCall(env, imsModRejectCaller, imsModRejectCallee)
	if err != nil {
		return err
	}

	defer c.close()

	var qfis []uint8

	steps := scenarios.IMSRejectedCallSteps{
		FirstUp: func(ctx context.Context, first scenarios.IMSMedia) error {
			var err error

			if qfis, err = awaitVoiceQFIs(ctx, c.legs); err != nil {
				return err
			}

			if err := sendVoiceBothWays(ctx, c.legs, qfis, first); err != nil {
				return err
			}

			c.ues[0].ue.RejectNextQoSRules()

			return nil
		},
		AfterRejection: func(ctx context.Context, first scenarios.IMSMedia) error {
			if err := requireSameVoiceQFIs(ctx, c.legs, qfis); err != nil {
				return fmt.Errorf("after the UE rejected the second call's QoS rule (TS 24.501 §6.3.2.5): %w", err)
			}

			return sendVoiceBothWays(ctx, c.legs, qfis, first)
		},
	}

	if err := scenarios.RequireIMSSecondCallRejected(ctx, c.endpoints[0], c.endpoints[1], scenarios.IMSTransports[0], steps); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, c.legs)
}

type flowLoss func(u imsCallUE, qfi uint8) error

func releaseFlowAtGNB(u imsCallUE, qfi uint8) error {
	l := u.leg

	return l.gnb.SendQoSFlowsReleased(l.gnb.GetAMFUENGAPID(l.ranUEID), l.ranUEID, int64(l.session.PDUSessionID), []uint8{qfi})
}

func loseRadioConnection(u imsCallUE, _ uint8) error {
	return u.leg.gnb.ReleaseContext(u.ue, u.leg.ranUEID, []uint8{u.leg.session.PDUSessionID}, gnb.CauseRadioConnectionWithUELost, releaseTimeout)
}

func runIMSCallLoss(ctx context.Context, env scenarios.Env, caller, callee scenarios.SubscriberSpec, lose flowLoss) error {
	c, err := setUpIMSCall(env, caller, callee)
	if err != nil {
		return err
	}

	defer c.close()

	loseFlow := func(ctx context.Context) error {
		qfis, err := awaitVoiceQFIs(ctx, c.legs)
		if err != nil {
			return err
		}

		return lose(c.ues[1], qfis[1])
	}

	if err := scenarios.RequireIMSCallLost(ctx, c.endpoints[0], c.endpoints[1], scenarios.IMSTransports[0], loseFlow); err != nil {
		return err
	}

	if err := awaitQoSFlowsReleased(ctx, c.legs[:1]); err != nil {
		return fmt.Errorf("the peer kept its voice flow after the call was lost: %w", err)
	}

	return nil
}

func runIMSCallIdle(ctx context.Context, env scenarios.Env) error {
	c, err := setUpIMSCall(env, imsIdleCaller, imsIdleCallee)
	if err != nil {
		return err
	}

	defer c.close()

	callee := c.ues[1]

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		qfis, err := awaitVoiceQFIs(ctx, c.legs)
		if err != nil {
			return err
		}

		if err := sendVoiceBothWays(ctx, c.legs, qfis, m); err != nil {
			return fmt.Errorf("before the release: %w", err)
		}

		if err := c.g.ReleaseContext(callee.ue, callee.leg.ranUEID, []uint8{callee.leg.session.PDUSessionID}, gnb.CauseUserInactivity, releaseTimeout); err != nil {
			return err
		}

		upf, err := netip.ParseAddr(c.legs[0].session.UpfAddress)
		if err != nil {
			return err
		}

		packet := scenarios.UDPPacket(netip.AddrPortFrom(m.Caller.Addr, m.Caller.Port), netip.AddrPortFrom(m.Callee.Addr, m.Callee.Port), voicePayload)
		if err := c.g.SendGPDUWithQFI(c.legs[0].session.ULTEID, upf, qfis[0], packet); err != nil {
			return err
		}

		if _, err := c.g.WaitForMessage(gnb.Initiating, ngaplib.ProcPaging, pagingTimeout); err != nil {
			return fmt.Errorf("voice for the idle callee did not page it (TS 23.502 §4.2.3.3): %w", err)
		}

		if _, err := c.g.ServiceRequest(callee.ue, callee.leg.ranUEID, callee.leg.session.PDUSessionID, releaseTimeout, &gnb.ServiceRequestOpts{
			DLTEID:      callee.leg.session.DLTEID,
			ServiceType: fgs.ServiceTypeMobileTerminatedServices,
		}); err != nil {
			return fmt.Errorf("answer the page: %w", err)
		}

		if flows := callee.leg.flows(); !slices.Equal(flows, qfis[1:]) {
			return fmt.Errorf("the reactivated session set up QoS flows %v, want the voice QFI %d kept across the user-inactivity release (TS 23.502 §4.2.6)", flows, qfis[1])
		}

		if err := sendVoiceBothWays(ctx, c.legs, qfis, m); err != nil {
			return fmt.Errorf("after the page: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSCallWithMedia(ctx, c.endpoints[0], c.endpoints[1], scenarios.IMSTransports[0], media); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, c.legs)
}

func rehomeIMSTunnel(env scenarios.Env, u imsCallUE, ep scenarios.IMSEndpoint, moved voiceLeg) error {
	u.leg.gnb.CloseTunnel(u.leg.session.DLTEID)

	if err := tunnelIMSSession(env, moved.gnb, moved.session, u.iface, scenarios.IMSUEIPv6Pool); err != nil {
		return fmt.Errorf("target gNB: %w", err)
	}

	return scenarios.RerouteToIMS(u.iface, ep.Local, ep.PCSCF, u.table)
}

func runIMSCallHandover(ctx context.Context, env scenarios.Env, caller, callee scenarios.SubscriberSpec, handOver func(target *gnb.GnodeB, n3 netip.Addr, u imsCallUE, qfi uint8) (voiceLeg, error)) error {
	spec := env.FirstGNB()
	if spec.N3Secondary == "" {
		return fmt.Errorf("a handover needs a secondary N3 address for the target gNB")
	}

	n3, err := netip.ParseAddr(spec.N3Secondary)
	if err != nil {
		return fmt.Errorf("parse the secondary N3 address: %w", err)
	}

	c, err := setUpIMSCall(env, caller, callee)
	if err != nil {
		return err
	}

	defer c.close()

	target, err := startXnTargetGNB(env, spec.N2Address, spec.N3Secondary)
	if err != nil {
		return err
	}

	defer target.Close()

	defer target.CloseTunnel(imsHandoverTargetTEID)

	legs := slices.Clone(c.legs)

	media := func(ctx context.Context, m scenarios.IMSMedia) error {
		qfis, err := awaitVoiceQFIs(ctx, legs)
		if err != nil {
			return err
		}

		if err := sendVoiceBothWays(ctx, legs, qfis, m); err != nil {
			return fmt.Errorf("before the handover: %w", err)
		}

		moved, err := handOver(target, n3, c.ues[1], qfis[1])
		if err != nil {
			return err
		}

		if err := rehomeIMSTunnel(env, c.ues[1], c.endpoints[1], moved); err != nil {
			return err
		}

		legs[1] = moved

		if err := requireSameVoiceQFIs(ctx, legs, qfis); err != nil {
			return fmt.Errorf("after the handover: %w", err)
		}

		if err := sendVoiceBothWays(ctx, legs, qfis, m); err != nil {
			return fmt.Errorf("after the handover: %w", err)
		}

		return nil
	}

	if err := scenarios.RequireIMSCallWithMedia(ctx, c.endpoints[0], c.endpoints[1], scenarios.IMSTransports[0], media); err != nil {
		return err
	}

	return awaitQoSFlowsReleased(ctx, legs)
}

func runIMSCallXnHandover(ctx context.Context, env scenarios.Env) error {
	return runIMSCallHandover(ctx, env, imsXnHandoverCaller, imsXnHandoverCallee, xnHandOverVoiceUE)
}

func runIMSCallN2Handover(ctx context.Context, env scenarios.Env) error {
	return runIMSCallHandover(ctx, env, imsN2HandoverCaller, imsN2HandoverCallee, n2HandOverVoiceUE)
}

func movedLeg(target *gnb.GnodeB, u imsCallUE, ul *ngaplib.UPTransportLayerInformation) voiceLeg {
	session := u.leg.session
	session.DLTEID = imsHandoverTargetTEID

	if ul != nil {
		session.ULTEID = uint32(ul.GTPTunnel.GTPTEID)
	}

	target.AddUE(imsHandoverTargetRANID, u.ue)
	u.ue.Gnb = target

	return voiceLeg{gnb: target, ranUEID: imsHandoverTargetRANID, session: session}
}

func xnHandOverVoiceUE(target *gnb.GnodeB, n3 netip.Addr, u imsCallUE, qfi uint8) (voiceLeg, error) {
	ack, err := xnPathSwitch(target, &xnPathSwitchOpts{
		SourceAMFUENGAPID: u.leg.gnb.GetAMFUENGAPID(u.leg.ranUEID),
		TargetRANUENGAPID: imsHandoverTargetRANID,
		TargetN3IP:        n3,
		TargetDLTEID:      imsHandoverTargetTEID,
		Flows:             []uint8{qfi},
	})
	if err != nil {
		return voiceLeg{}, fmt.Errorf("handover over Xn: %w", err)
	}

	transfer, err := ngaplib.ParsePathSwitchRequestAcknowledgeTransfer(ack.PDUSessionResourceSwitchedList[0].Transfer)
	if err != nil {
		return voiceLeg{}, fmt.Errorf("parse the Path Switch Request Acknowledge Transfer: %w", err)
	}

	return movedLeg(target, u, transfer.ULNGUUPTNLInformation), nil
}

func n2HandOverVoiceUE(target *gnb.GnodeB, n3 netip.Addr, u imsCallUE, qfi uint8) (voiceLeg, error) {
	source := u.leg.gnb
	sourceAMFUEID := source.GetAMFUENGAPID(u.leg.ranUEID)

	if err := source.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  sourceAMFUEID,
		RANUENGAPID:  u.leg.ranUEID,
		HandoverType: ngaplib.HandoverTypeIntra5GS,
		TargetGnbID:  handoverTargetGnbID,
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(u.leg.session.PDUSessionID), HandoverRequiredTransfer: directForwardingRequiredTransfer},
		},
		SourceToTargetTransparentContainer: n2SourceToTargetContainer,
	}); err != nil {
		return voiceLeg{}, fmt.Errorf("send Handover Required: %w", err)
	}

	frame, err := target.WaitForMessage(gnb.Initiating, ngaplib.ProcHandoverResourceAllocation, 5*time.Second)
	if err != nil {
		return voiceLeg{}, fmt.Errorf("target gNB: await Handover Request: %w", err)
	}

	req, err := ngaplib.ParseHandoverRequest(frame.Value)
	if err != nil {
		return voiceLeg{}, fmt.Errorf("parse Handover Request: %w", err)
	}

	if len(req.PDUSessionResourceSetupListHOReq) != 1 {
		return voiceLeg{}, fmt.Errorf("the Handover Request sets up %d PDU sessions, want 1", len(req.PDUSessionResourceSetupListHOReq))
	}

	transfer, err := ngaplib.ParsePDUSessionResourceSetupRequestTransfer(req.PDUSessionResourceSetupListHOReq[0].Transfer)
	if err != nil {
		return voiceLeg{}, fmt.Errorf("parse the Handover Request's session transfer: %w", err)
	}

	if !slices.ContainsFunc(transfer.QosFlowSetupRequest, func(i ngaplib.QosFlowSetupRequestItem) bool {
		return uint8(i.QosFlowIdentifier) == qfi && i.QosFlowLevelQosParameters.GBRQosInformation != nil
	}) {
		return voiceLeg{}, fmt.Errorf("the Handover Request flows %+v lack the voice QFI %d with its GBR (TS 23.502 §4.9.1.3.2 step 9)", transfer.QosFlowSetupRequest, qfi)
	}

	targetAMFUEID, err := common.ExtractAmfUeNgapIDFromHandoverRequest(frame.Data)
	if err != nil {
		return voiceLeg{}, fmt.Errorf("extract AMF UE NGAP ID from Handover Request: %w", err)
	}

	if err := target.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: targetAMFUEID,
		RANUENGAPID: imsHandoverTargetRANID,
		PDUSessions: []gnb.HandoverAdmittedPDUSession{{
			PDUSessionID: int64(u.leg.session.PDUSessionID),
			DLTEID:       imsHandoverTargetTEID,
			DLIP:         n3,
			QFIs:         []uint8{qfi},
		}},
		TargetToSourceTransparentContainer: n2HandoverRRCContainer,
	}); err != nil {
		return voiceLeg{}, fmt.Errorf("send Handover Request Acknowledge: %w", err)
	}

	if _, err := source.WaitForMessage(gnb.Successful, ngaplib.ProcHandoverPreparation, 5*time.Second); err != nil {
		return voiceLeg{}, fmt.Errorf("source gNB: await Handover Command: %w", err)
	}

	if err := relayRANStatusTransfer(source, target, sourceAMFUEID, u.leg.ranUEID, imsHandoverTargetRANID); err != nil {
		return voiceLeg{}, err
	}

	moved := movedLeg(target, u, &transfer.ULNGUUPTNLInformation)

	if err := target.SendHandoverNotify(&gnb.HandoverNotifyOpts{AMFUENGAPID: targetAMFUEID, RANUENGAPID: imsHandoverTargetRANID}); err != nil {
		return voiceLeg{}, fmt.Errorf("send Handover Notify: %w", err)
	}

	if _, err := source.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 5*time.Second); err != nil {
		return voiceLeg{}, fmt.Errorf("source gNB: await UE Context Release Command: %w", err)
	}

	return moved, nil
}
