// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// standaloneBufferUE builds a secured, registered UE with a paging procedure in flight,
// ready to answer with an MT service request.
func standaloneBufferUE(t *testing.T) (*amf.AMF, *amf.UeContext, *fakeNGAPSender, [16]uint8, nas.CipheringAlgorithm) {
	t.Helper()

	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: "[\"000001\"]"},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{},
	)
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ArmPagingForTest(6*time.Minute, 5)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(mustTestGuti("001", "01", "cafe42", 0x00000001))
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)

	ng := ue.NgKsiForTest()
	ng.Ksi = 1
	ue.SetNgKsiForTest(ng)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(nas.CipheringAES)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}

	return amfInstance, ue, ngapSender, key, nas.CipheringAES
}

// answerPaging drives an MT service request, the branch a paged UE takes.
func answerPaging(t *testing.T, amfInstance *amf.AMF, ue *amf.UeContext, algo nas.CipheringAlgorithm, key [16]uint8) {
	t.Helper()

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)
}

// A UE answering a page for a buffered LPP message is sent that message in a DL NAS
// Transport with the LPP payload container and the LCS correlation identifier in the
// Additional information IE (TS 23.273 §6.11.1 step 3, TS 24.501 §5.4.5.3.1 case c).
func TestHandleServiceRequest_MT_BufferedLPP_Delivered(t *testing.T) {
	amfInstance, ue, ngapSender, key, algo := standaloneBufferUE(t)

	lppMsg := []byte{0x11, 0x22, 0x33}
	correlID := []byte{0xde, 0xad, 0xbe, 0xef}

	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		N1Class:             models.N1ClassLPP,
		BinaryDataN1Message: lppMsg,
		LCSCorrelationID:    correlID,
	})

	answerPaging(t, amfInstance, ue, algo, key)

	// With no PDU session to reactivate the Service Accept goes out on its own, so the
	// buffered message follows it: Service Accept, buffered LPP, Configuration Update
	// Command (the GUTI reallocation an MT service request triggers).
	if len(ngapSender.SentDownlinkNASTransport) != 3 {
		t.Fatalf("DL NAS Transports sent = %d, want 3", len(ngapSender.SentDownlinkNASTransport))
	}

	decipherGmm(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgServiceAccept))
	plain := decipherGmmCount(t, ue, ngapSender.SentDownlinkNASTransport[1].NASPDU, ue.ULCount()+1, uint8(fgs.MsgDLNASTransport))
	decipherGmmCount(t, ue, ngapSender.SentDownlinkNASTransport[2].NASPDU, ue.ULCount()+2, uint8(fgs.MsgConfigurationUpdateCommand))

	dl, err := fgs.ParseDLNASTransport(plain)
	if err != nil {
		t.Fatalf("could not parse DL NAS transport: %v", err)
	}

	if dl.PayloadContainerType != fgs.PayloadContainerTypeLPP {
		t.Errorf("payload container type = %v, want LPP", dl.PayloadContainerType)
	}

	if !bytes.Equal(dl.PayloadContainer, lppMsg) {
		t.Errorf("payload = %x, want %x", dl.PayloadContainer, lppMsg)
	}

	if !bytes.Equal(dl.AdditionalInfo, correlID) {
		t.Errorf("additional information = %x, want the correlation id %x", dl.AdditionalInfo, correlID)
	}

	if ue.N1N2Message() != nil {
		t.Error("expected the buffer to be cleared once delivered")
	}
}

// The NRPPa half goes to the RAN as a Downlink UE-Associated NRPPa Transport rather than to
// the UE, and no PDU session resources are touched (TS 23.273 §6.11.2 step 3).
func TestHandleServiceRequest_MT_BufferedNRPPa_Delivered(t *testing.T) {
	amfInstance, ue, ngapSender, key, algo := standaloneBufferUE(t)

	nrppaPdu := []byte{0x0a, 0x0b, 0x0c}

	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		N2Class:                 models.N2ClassNRPPa,
		BinaryDataN2Information: nrppaPdu,
		RoutingID:               5,
	})

	answerPaging(t, amfInstance, ue, algo, key)

	if len(ngapSender.SentDownlinkUEAssociatedNRPPaTransport) != 1 {
		t.Fatalf("NRPPa transports sent = %d, want 1", len(ngapSender.SentDownlinkUEAssociatedNRPPaTransport))
	}

	sent := ngapSender.SentDownlinkUEAssociatedNRPPaTransport[0]
	if !bytes.Equal(sent.NRPPaPDU, nrppaPdu) {
		t.Errorf("NRPPa PDU = %x, want %x", sent.NRPPaPDU, nrppaPdu)
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Error("a positioning buffer must not drive PDU session resource setup")
	}

	if ue.N1N2Message() != nil {
		t.Error("expected the buffer to be cleared once delivered")
	}
}
