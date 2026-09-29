// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

const smsTestIMSI = "001019756139935"

type fakeSMSHandler struct {
	mu      sync.Mutex
	uplinks [][]byte
}

func (h *fakeSMSHandler) Allowed(context.Context, string) (bool, error) { return true, nil }

func (h *fakeSMSHandler) Uplink(_ context.Context, _ string, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.uplinks = append(h.uplinks, payload)
}

func (h *fakeSMSHandler) UEReachable(context.Context, string) {}

func (h *fakeSMSHandler) Activate(_ string, _ any) {}

func (h *fakeSMSHandler) Deactivate(_ string, _ any) {}

func (h *fakeSMSHandler) DeliveryFailed(string) {}

func (h *fakeSMSHandler) TransactionPending(string) bool { return false }

func smsTestAMF(t *testing.T) (*amf.AMF, *amf.UeContext, [16]uint8, nas.CipheringAlgorithm) {
	t.Helper()

	amfInstance, ue, _, key, algo := standaloneBufferUE(t)
	ue.StopPagingForTest(t.Context())
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-" + smsTestIMSI))

	return amfInstance, ue, key, algo
}

func pagedForSMS(t *testing.T) (*amf.AMF, *amf.UeContext, *fakeNGAPSender, [16]uint8, nas.CipheringAlgorithm, <-chan error) {
	t.Helper()

	amfInstance, ue, key, algo := smsTestAMF(t)
	amfInstance.SMS = &fakeSMSHandler{}

	if err := amfInstance.CommitUEIdentity(t.Context(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	tai := ue.Conn().Tai
	ue.AllocateRegistrationArea([]models.Tai{tai})
	amfInstance.GrantSMSOverNAS(t.Context(), ue, true)
	ue.Conn().Release(t.Context())

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{Tai: tai}})

	reached := make(chan error, 1)

	go func() { reached <- amfInstance.EnableUEReachabilityForSMS(context.Background(), smsTestIMSI) }()

	deadline := time.Now().Add(time.Second)
	for ue.PagingState() != amf.PagingAttempting {
		if time.Now().After(deadline) {
			t.Fatal("the idle UE was not paged")
		}

		time.Sleep(time.Millisecond)
	}

	conn, err := amfInstance.NewUeConn(radio, 1)
	if err != nil {
		t.Fatalf("NewUeConn: %v", err)
	}

	conn.Tai = tai
	amfInstance.AttachUeConn(t.Context(), ue, conn)

	return amfInstance, ue, sender, key, algo, reached
}

func awaitReachable(t *testing.T, reached <-chan error) {
	t.Helper()

	select {
	case err := <-reached:
		if err != nil {
			t.Fatalf("EnableUEReachabilityForSMS: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the UE was not reported reachable after answering the page")
	}
}

func TestMobileTerminatedSMSToAnIdleUEFollowsTheServiceRequest(t *testing.T) {
	amfInstance, ue, sender, key, algo, reached := pagedForSMS(t)

	answerPaging(t, amfInstance, ue, algo, key)
	awaitReachable(t, reached)

	cpData := []byte{0x09, 0x01, 0x02, 0x03}

	if err := amfInstance.SendSMS(t.Context(), smsTestIMSI, cpData); err != nil {
		t.Fatalf("SendSMS: %v", err)
	}

	if len(sender.SentDownlinkNASTransport) != 3 {
		t.Fatalf("DL NAS Transports sent = %d, want 3", len(sender.SentDownlinkNASTransport))
	}

	decipherGmm(t, ue, sender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgServiceAccept))
	decipherGmmCount(t, ue, sender.SentDownlinkNASTransport[1].NASPDU, ue.ULCount()+1, uint8(fgs.MsgConfigurationUpdateCommand))
	plain := decipherGmmCount(t, ue, sender.SentDownlinkNASTransport[2].NASPDU, ue.ULCount()+2, uint8(fgs.MsgDLNASTransport))

	dl, err := fgs.ParseDLNASTransport(plain)
	if err != nil {
		t.Fatalf("parse DL NAS Transport: %v", err)
	}

	if dl.PayloadContainerType != fgs.PayloadContainerTypeSMS || !bytes.Equal(dl.PayloadContainer, cpData) {
		t.Fatalf("DL NAS Transport carries %v %x, want SMS %x", dl.PayloadContainerType, dl.PayloadContainer, cpData)
	}
}

func TestAnSMSPageAnsweredByASignallingServiceRequestSettles(t *testing.T) {
	amfInstance, ue, _, key, algo, reached := pagedForSMS(t)

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeSignalling)
	if err != nil {
		t.Fatal(err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	awaitReachable(t, reached)

	if ue.PagingState() != amf.PagingIdle {
		t.Fatalf("paging state = %s, want Idle", ue.PagingState())
	}
}

func TestUplinkSMSIsHandedToTheSMSF(t *testing.T) {
	amfInstance, ue, _, _ := smsTestAMF(t)
	handler := &fakeSMSHandler{}
	amfInstance.SMS = handler

	cpData := []byte{0x09, 0x01, 0x02}

	handleULNASTransport(t.Context(), amfInstance, ue, &fgs.ULNASTransport{PayloadContainerType: fgs.PayloadContainerTypeSMS, PayloadContainer: cpData})

	amfInstance.GrantSMSOverNAS(t.Context(), ue, true)
	handleULNASTransport(t.Context(), amfInstance, ue, &fgs.ULNASTransport{PayloadContainerType: fgs.PayloadContainerTypeSMS, PayloadContainer: cpData})

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.uplinks) != 1 || !bytes.Equal(handler.uplinks[0], cpData) {
		t.Fatalf("uplinks = %x, want only the one sent after SMS over NAS was allowed", handler.uplinks)
	}
}

func TestEmergencyRegistrationIsNotGrantedSMS(t *testing.T) {
	amfInstance, ue, _, _ := smsTestAMF(t)
	amfInstance.SMS = &fakeSMSHandler{}

	contextSetup(t.Context(), amfInstance, ue, &fgs.RegistrationRequest{
		RegistrationType: fgs.RegistrationTypeEmergency,
		UpdateType5GS:    &fgs.UpdateType5GS{SMSRequested: true},
	}, nil)

	if ue.SMSOverNAS() {
		t.Fatal("an emergency registration was granted SMS over NAS")
	}
}
