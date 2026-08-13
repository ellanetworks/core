// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

const fcKASMEPrimeIdle = "73"

type IdleMobilityFrom5GS struct {
	KAMF             []byte
	KNASInt          [16]byte
	NIA              uint8
	UplinkNASCount   nas.Count
	DownlinkNASCount nas.Count
	EPSCiphering     uint8
	EPSIntegrity     uint8
	EKSI             uint8
}

func (ue *UE) InstallMappedSecurityContextForIdleMobility(in IdleMobilityFrom5GS) error {
	if len(in.KAMF) != 32 {
		return fmt.Errorf("s1enb: K_AMF is %d octets, want 32", len(in.KAMF))
	}

	p0 := make([]byte, 4)
	binary.BigEndian.PutUint32(p0, in.UplinkNASCount.Value())

	kasme, err := ueauth.GetKDFValue(in.KAMF, fcKASMEPrimeIdle, p0, ueauth.KDFLen(p0))
	if err != nil {
		return fmt.Errorf("s1enb: derive K'ASME: %w", err)
	}

	ue.kasme = kasme
	ue.eea, ue.eia = in.EPSCiphering, in.EPSIntegrity

	if err := ue.deriveNASKeys(); err != nil {
		return err
	}

	ue.ulCount = in.UplinkNASCount.Next().SQN()
	ue.dlCount.seed(in.DownlinkNASCount)

	return nil
}

type IdleTrackingAreaUpdateOpts struct {
	GUTI         eps.GUTI
	ActiveFlag   bool
	BearerStatus *nas.EPSBearerContextStatus
	Security     IdleMobilityFrom5GS
}

func (ue *UE) BuildIdleTrackingAreaUpdate(opts IdleTrackingAreaUpdateOpts) ([]byte, error) {
	gutiType := eps.GUTITypeNative

	plain, err := (&eps.TrackingAreaUpdateRequest{
		EPSUpdateType:          eps.EPSUpdateTypeTA,
		ActiveFlag:             opts.ActiveFlag,
		NASKeySetIdentifier:    nas.KeySetIdentifier{Value: opts.Security.EKSI, Mapped: true},
		OldGUTI:                eps.GUTIIdentity(opts.GUTI),
		OldGUTIType:            &gutiType,
		UEStatus:               &eps.UEStatus{N1ModeReg: true},
		EPSBearerContextStatus: opts.BearerStatus,
		UENetworkCapability:    &eps.UENetworkCapability{EEA: ue.netCapEEA, EIA: ue.netCapEIA},
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("s1enb: build Tracking Area Update Request: %w", err)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:          nas.IntegrityAlgorithm(opts.Security.NIA),
		IntegrityKey:       opts.Security.KNASInt,
		Ciphering:          nas.CipheringNull,
		AllowNullIntegrity: nas.IntegrityAlgorithm(opts.Security.NIA) == nas.IntegrityNull,
	})
	if err != nil {
		return nil, fmt.Errorf("s1enb: build the 5G NAS security context: %w", err)
	}

	count := opts.Security.UplinkNASCount

	mac, err := sc.MAC(append([]byte{count.SQN()}, plain...), count, nas.Bearer3GPP, nas.DirectionUplink)
	if err != nil {
		return nil, fmt.Errorf("s1enb: MAC the Tracking Area Update Request: %w", err)
	}

	wire, err := (&eps.SecurityProtectedMessage{
		SecurityHeaderType: eps.SHTIntegrityProtected,
		MAC:                mac,
		SequenceNumber:     count.SQN(),
		UnverifiedPayload:  plain,
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("s1enb: frame the Tracking Area Update Request: %w", err)
	}

	if err := ue.InstallMappedSecurityContextForIdleMobility(opts.Security); err != nil {
		return nil, err
	}

	return wire, nil
}

func (e *ENB) TrackingAreaUpdateFrom5GS(ue *UE, opts IdleTrackingAreaUpdateOpts, timeout time.Duration) (*AttachResult, error) {
	enbUEID := e.AllocateENBUEID()

	wire, err := ue.BuildIdleTrackingAreaUpdate(opts)
	if err != nil {
		return nil, err
	}

	if err := e.SendInitialUEMessage(enbUEID, wire); err != nil {
		return nil, fmt.Errorf("s1enb: send the Tracking Area Update Request: %w", err)
	}

	if !opts.ActiveFlag {
		return nil, fmt.Errorf("s1enb: an idle-mode update without the active flag establishes no bearer to report")
	}

	icsFrame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcInitialContextSetup, timeout)
	if err != nil {
		return nil, fmt.Errorf("s1enb: await Initial Context Setup Request: %w", err)
	}

	ics, err := s1ap.ParseInitialContextSetupRequest(icsFrame.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse Initial Context Setup Request: %w", err)
	}

	if len(ics.ERABToBeSetup) == 0 {
		return nil, fmt.Errorf("s1enb: initial context setup request without an E-RAB")
	}

	erab := ics.ERABToBeSetup[0]

	upf, err := e.selectUpfAddr(erab.TransportLayerAddress)
	if err != nil {
		return nil, err
	}

	dlTEID := e.allocTEID()

	if err := e.sendInitialContextSetupResponse(ics, enbUEID, dlTEID); err != nil {
		return nil, err
	}

	acceptPlain, err := ue.unprotectDownlink([]byte(erab.NASPDU))
	if err != nil {
		return nil, fmt.Errorf("s1enb: unprotect Tracking Area Update Accept: %w", err)
	}

	accept, err := expectDownlink[*eps.TrackingAreaUpdateAccept](acceptPlain)
	if err != nil {
		return nil, fmt.Errorf("s1enb: await Tracking Area Update Accept: %w", err)
	}

	mmeUEID := int64(ics.MMEUES1APID)

	complete, err := ue.buildTrackingAreaUpdateComplete()
	if err != nil {
		return nil, err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, complete); err != nil {
		return nil, fmt.Errorf("s1enb: send Tracking Area Update Complete: %w", err)
	}

	logger.GnbLogger.Debug("Tracking area update from 5GS complete",
		zap.String("imsi", ue.IMSI), zap.Int64("mme-ue-id", mmeUEID), zap.Int64("enb-ue-id", enbUEID))

	return &AttachResult{
		MMEUES1APID:  mmeUEID,
		ENBUES1APID:  enbUEID,
		ERABID:       erab.ERABID,
		GUTI:         accept.GUTI,
		UpfAddress:   upf.Unmap().String(),
		ULTEID:       uint32(erab.GTPTEID),
		DLTEID:       dlTEID,
		BearerStatus: accept.EPSBearerContextStatus,
	}, nil
}
