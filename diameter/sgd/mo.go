// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type MOForwardShortMessage struct {
	ServiceCentreAddress string
	User                 tgpp.UserIdentifier
	SMRPUI               []byte
}

var ofrRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: tgpp.AVPSCAddress, VendorID: tgpp.VendorID}:          {Required: true},
	{Code: AVPOFRFlags, VendorID: tgpp.VendorID}:                {},
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}:  {Multiple: true},
	{Code: tgpp.AVPUserIdentifier, VendorID: tgpp.VendorID}:     {Required: true},
	{Code: AVPEPSLocationInformation, VendorID: tgpp.VendorID}:  {},
	{Code: AVPNRCellGlobalIdentity, VendorID: tgpp.VendorID}:    {},
	{Code: AVPSMRPUI, VendorID: tgpp.VendorID}:                  {Required: true},
	{Code: tgpp.AVPSMSMICorrelationID, VendorID: tgpp.VendorID}: {},
	{Code: tgpp.AVPSMDeliveryOutcome, VendorID: tgpp.VendorID}:  {},
	{Code: AVPMPSPriority, VendorID: tgpp.VendorID}:             {},
})

func NewMOForwardShortMessageRequest(env tgpp.Envelope, m MOForwardShortMessage) (*diameter.Message, error) {
	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

	scAddress, err := tgpp.EncodeE164(m.ServiceCentreAddress)
	if err != nil {
		return nil, invalidf("service centre address: %w", err)
	}

	ui, err := tgpp.NewUserIdentifier(m.User)
	if err != nil {
		return nil, invalidf("%w", err)
	}

	if err := checkSMRPUI(m.SMRPUI); err != nil {
		return nil, err
	}

	avps := append(env.AVPs(),
		diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, scAddress),
		ui,
		smRPUIAVP(m.SMRPUI),
	)

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   CommandMOForwardShortMessage,
		ApplicationID: ApplicationID,
		AVPs:          avps,
	}, nil
}

func CheckMOForwardShortMessage(req *diameter.Message) error {
	return ofrRules.Check(req)
}

func ParseMOForwardShortMessageRequest(req *diameter.Message) (MOForwardShortMessage, error) {
	if err := CheckMOForwardShortMessage(req); err != nil {
		return MOForwardShortMessage{}, err
	}

	var (
		m   MOForwardShortMessage
		err error
	)

	sc, _ := req.Find(tgpp.AVPSCAddress, tgpp.VendorID)
	if m.ServiceCentreAddress, err = tgpp.DecodeE164(sc.Data); err != nil {
		return MOForwardShortMessage{}, tgpp.InvalidAVP(sc)
	}

	ui, _ := req.Find(tgpp.AVPUserIdentifier, tgpp.VendorID)
	if m.User, err = tgpp.ParseUserIdentifier(ui); err != nil {
		return MOForwardShortMessage{}, tgpp.InvalidAVP(ui)
	}

	smRPUI, _ := req.Find(AVPSMRPUI, tgpp.VendorID)
	if checkSMRPUI(smRPUI.Data) != nil {
		return MOForwardShortMessage{}, tgpp.InvalidAVP(smRPUI)
	}

	m.SMRPUI = smRPUI.Data

	return m, nil
}

func NewMOForwardShortMessageAnswer(req *diameter.Message, id diameter.Identity, smRPUI []byte) (*diameter.Message, error) {
	return newSuccessAnswer(req, id, smRPUI)
}

func newSuccessAnswer(req *diameter.Message, id diameter.Identity, smRPUI []byte) (*diameter.Message, error) {
	ans := tgpp.NewAnswer(req, id, diameter.ResultSuccess)

	if smRPUI != nil {
		if err := checkSMRPUI(smRPUI); err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, smRPUIAVP(smRPUI))
	}

	return ans, nil
}
