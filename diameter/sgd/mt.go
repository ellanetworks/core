// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type MTForwardShortMessage struct {
	IMSI                 string
	ServiceCentreAddress string
	SMRPUI               []byte
	MMENumberForMTSMS    string
	SGSNNumber           string
	MoreMessagesToSend   bool
	DeliveryTimer        time.Duration
	DeliveryStartTime    time.Time
}

var tfrRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: diameter.AVPDestinationHost}:                           {Required: true},
	{Code: diameter.AVPUserName}:                                  {Required: true},
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}:    {Multiple: true},
	{Code: tgpp.AVPSMSMICorrelationID, VendorID: tgpp.VendorID}:   {},
	{Code: tgpp.AVPSCAddress, VendorID: tgpp.VendorID}:            {Required: true},
	{Code: AVPSMRPUI, VendorID: tgpp.VendorID}:                    {Required: true},
	{Code: tgpp.AVPMMENumberForMTSMS, VendorID: tgpp.VendorID}:    {},
	{Code: tgpp.AVPSGSNNumber, VendorID: tgpp.VendorID}:           {},
	{Code: AVPTFRFlags, VendorID: tgpp.VendorID}:                  {},
	{Code: AVPSMDeliveryTimer, VendorID: tgpp.VendorID}:           {},
	{Code: AVPSMDeliveryStartTime, VendorID: tgpp.VendorID}:       {},
	{Code: AVPMaximumRetransmissionTime, VendorID: tgpp.VendorID}: {},
	{Code: AVPSMSGMSCAddress, VendorID: tgpp.VendorID}:            {},
	{Code: AVPMPSPriority, VendorID: tgpp.VendorID}:               {},
})

func NewMTForwardShortMessageRequest(env tgpp.Envelope, m MTForwardShortMessage) (*diameter.Message, error) {
	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

	if env.DestinationHost == "" {
		return nil, invalidf("MT-Forward-Short-Message-Request without a Destination-Host")
	}

	if !tgpp.ValidIMSI(m.IMSI) {
		return nil, invalidf("IMSI %q", m.IMSI)
	}

	if err := checkSMRPUI(m.SMRPUI); err != nil {
		return nil, err
	}

	if m.DeliveryTimer < 0 || m.DeliveryTimer/time.Second > 1<<32-1 {
		return nil, invalidf("SM-Delivery-Timer %s", m.DeliveryTimer)
	}

	scAddress, err := tgpp.EncodeE164(m.ServiceCentreAddress)
	if err != nil {
		return nil, invalidf("service centre address: %w", err)
	}

	avps := append(env.AVPs(),
		diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, m.IMSI),
		diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, scAddress),
		smRPUIAVP(m.SMRPUI),
	)

	for _, n := range []struct {
		code  uint32
		value string
	}{
		{tgpp.AVPMMENumberForMTSMS, m.MMENumberForMTSMS},
		{tgpp.AVPSGSNNumber, m.SGSNNumber},
	} {
		if n.value == "" {
			continue
		}

		number, err := tgpp.EncodeE164(n.value)
		if err != nil {
			return nil, invalidf("serving node number: %w", err)
		}

		avps = append(avps, diameter.OctetString(n.code, 0, tgpp.VendorID, number))
	}

	if m.MoreMessagesToSend {
		avps = append(avps, diameter.Unsigned32(AVPTFRFlags, diameter.AVPFlagMandatory, tgpp.VendorID, tfrFlagMoreMessagesToSend))
	}

	if m.DeliveryTimer > 0 {
		avps = append(avps, diameter.Unsigned32(AVPSMDeliveryTimer, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(m.DeliveryTimer/time.Second)))
	}

	if !m.DeliveryStartTime.IsZero() {
		avps = append(avps, diameter.Time(AVPSMDeliveryStartTime, diameter.AVPFlagMandatory, tgpp.VendorID, m.DeliveryStartTime))
	}

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   CommandMTForwardShortMessage,
		ApplicationID: ApplicationID,
		AVPs:          avps,
	}, nil
}

func CheckMTForwardShortMessage(req *diameter.Message) error {
	return tfrRules.Check(req)
}

func ParseMTForwardShortMessageRequest(req *diameter.Message) (MTForwardShortMessage, error) {
	if err := CheckMTForwardShortMessage(req); err != nil {
		return MTForwardShortMessage{}, err
	}

	var (
		m   MTForwardShortMessage
		err error
	)

	userName, _ := req.Find(diameter.AVPUserName, 0)
	if m.IMSI = userName.UTF8String(); !tgpp.ValidIMSI(m.IMSI) {
		return MTForwardShortMessage{}, tgpp.InvalidAVP(userName)
	}

	sc, _ := req.Find(tgpp.AVPSCAddress, tgpp.VendorID)
	if m.ServiceCentreAddress, err = tgpp.DecodeE164(sc.Data); err != nil {
		return MTForwardShortMessage{}, tgpp.InvalidAVP(sc)
	}

	smRPUI, _ := req.Find(AVPSMRPUI, tgpp.VendorID)
	if checkSMRPUI(smRPUI.Data) != nil {
		return MTForwardShortMessage{}, tgpp.InvalidAVP(smRPUI)
	}

	m.SMRPUI = smRPUI.Data

	for _, n := range []struct {
		code uint32
		dst  *string
	}{
		{tgpp.AVPMMENumberForMTSMS, &m.MMENumberForMTSMS},
		{tgpp.AVPSGSNNumber, &m.SGSNNumber},
	} {
		if a, ok := req.Find(n.code, tgpp.VendorID); ok {
			if *n.dst, err = tgpp.DecodeE164(a.Data); err != nil {
				return MTForwardShortMessage{}, tgpp.InvalidAVP(a)
			}
		}
	}

	if a, ok := req.Find(AVPTFRFlags, tgpp.VendorID); ok {
		flags, err := tgpp.Unsigned32(a)
		if err != nil {
			return MTForwardShortMessage{}, err
		}

		m.MoreMessagesToSend = flags&tfrFlagMoreMessagesToSend != 0
	}

	if a, ok := req.Find(AVPSMDeliveryTimer, tgpp.VendorID); ok {
		seconds, err := tgpp.Unsigned32(a)
		if err != nil {
			return MTForwardShortMessage{}, err
		}

		m.DeliveryTimer = time.Duration(seconds) * time.Second
	}

	if a, ok := req.Find(AVPSMDeliveryStartTime, tgpp.VendorID); ok {
		if m.DeliveryStartTime, err = a.Time(); err != nil {
			return MTForwardShortMessage{}, tgpp.InvalidAVP(a)
		}
	}

	return m, nil
}

func NewMTForwardShortMessageAnswer(req *diameter.Message, id diameter.Identity, smRPUI []byte) (*diameter.Message, error) {
	return newSuccessAnswer(req, id, smRPUI)
}

func NewAbsentUserAnswer(req *diameter.Message, id diameter.Identity, diagnostic *tgpp.AbsentUserDiagnostic) *diameter.Message {
	ans := tgpp.NewExperimentalAnswer(req, id, tgpp.ResultErrorAbsentUser)

	if diagnostic != nil {
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(tgpp.AVPAbsentUserDiagnosticSM, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(*diagnostic)))
	}

	return ans
}
