// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type Alert struct {
	ServiceCentreAddress      string
	User                      tgpp.UserIdentifier
	MaximumUEAvailabilityTime time.Time
	Event                     AlertEvent
	ServingNode               *ServingNode
}

const alertEvents = AlertEventUEAvailableForMTSMS | AlertEventUEUnderNewNode

var alrRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: tgpp.AVPSCAddress, VendorID: tgpp.VendorID}:            {Required: true},
	{Code: tgpp.AVPUserIdentifier, VendorID: tgpp.VendorID}:       {Required: true},
	{Code: tgpp.AVPSMSMICorrelationID, VendorID: tgpp.VendorID}:   {},
	{Code: AVPMaximumUEAvailabilityTime, VendorID: tgpp.VendorID}: {},
	{Code: AVPSMSGMSCAlertEvent, VendorID: tgpp.VendorID}:         {},
	{Code: AVPServingNode, VendorID: tgpp.VendorID}:               {},
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}:    {Multiple: true},
})

func NewHSSAlertServiceCentreRequest(env tgpp.Envelope, a Alert) (*diameter.Message, error) {
	switch {
	case a.User.MSISDN == "":
		return nil, invalid("HSS alert without an MSISDN")
	case a.User.IMSI != "":
		return nil, invalid("HSS alert with an IMSI")
	case a.Event != 0 || a.ServingNode != nil:
		return nil, invalid("HSS alert with an SMS-GMSC alert event or serving node")
	}

	return newAlertServiceCentreRequest(env, a)
}

func NewMMEAlertServiceCentreRequest(env tgpp.Envelope, a Alert) (*diameter.Message, error) {
	switch {
	case a.User.IMSI == "" || a.User.MSISDN != "":
		return nil, invalid("MME alert must identify the user by IMSI only")
	case a.Event == 0 || a.Event&^alertEvents != 0:
		return nil, invalid("SMS-GMSC-Alert-Event %#x", uint32(a.Event))
	case a.ServingNode != nil && a.Event&AlertEventUEUnderNewNode == 0:
		return nil, invalid("new serving node without the UE-under-new-node event")
	case a.ServingNode != nil && (a.ServingNode.MSCNumber != "" || a.ServingNode.IPSMGW != nil):
		return nil, invalid("new serving node must be an MME or an SGSN")
	}

	if a.ServingNode != nil {
		if err := a.ServingNode.validate(false); err != nil {
			return nil, err
		}
	}

	return newAlertServiceCentreRequest(env, a)
}

func newAlertServiceCentreRequest(env tgpp.Envelope, a Alert) (*diameter.Message, error) {
	scAddress, err := tgpp.EncodeE164(a.ServiceCentreAddress)
	if err != nil {
		return nil, invalid("service centre address: %w", err)
	}

	ui, err := tgpp.NewUserIdentifier(a.User)
	if err != nil {
		return nil, invalid("%w", err)
	}

	avps := append(env.AVPs(),
		diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, scAddress),
		ui,
	)

	if !a.MaximumUEAvailabilityTime.IsZero() {
		avps = append(avps, diameter.Time(AVPMaximumUEAvailabilityTime, 0, tgpp.VendorID, a.MaximumUEAvailabilityTime))
	}

	if a.Event != 0 {
		avps = append(avps, diameter.Unsigned32(AVPSMSGMSCAlertEvent, 0, tgpp.VendorID, uint32(a.Event)))
	}

	if a.ServingNode != nil {
		node, err := servingNodeAVP(AVPServingNode, *a.ServingNode)
		if err != nil {
			return nil, err
		}

		avps = append(avps, node)
	}

	avps = append(avps, smsfSupportFeature.AVP())

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   CommandAlertServiceCentre,
		ApplicationID: ApplicationID,
		AVPs:          avps,
	}, nil
}

func CheckAlertServiceCentre(req *diameter.Message) error {
	return alrRules.Check(req)
}

func ParseAlertServiceCentreRequest(req *diameter.Message) (Alert, error) {
	if err := CheckAlertServiceCentre(req); err != nil {
		return Alert{}, err
	}

	var (
		a   Alert
		err error
	)

	sc, _ := req.Find(tgpp.AVPSCAddress, tgpp.VendorID)
	if a.ServiceCentreAddress, err = tgpp.DecodeE164(sc.Data); err != nil {
		return Alert{}, tgpp.InvalidAVP(sc)
	}

	ui, _ := req.Find(tgpp.AVPUserIdentifier, tgpp.VendorID)
	if a.User, err = tgpp.ParseUserIdentifier(ui); err != nil {
		return Alert{}, tgpp.InvalidAVP(ui)
	}

	if t, ok := req.Find(AVPMaximumUEAvailabilityTime, tgpp.VendorID); ok {
		if a.MaximumUEAvailabilityTime, err = t.Time(); err != nil {
			return Alert{}, tgpp.InvalidAVP(t)
		}
	}

	if e, ok := req.Find(AVPSMSGMSCAlertEvent, tgpp.VendorID); ok {
		event, err := e.Unsigned32()
		if err != nil {
			return Alert{}, tgpp.InvalidAVP(e)
		}

		a.Event = AlertEvent(event) & alertEvents
	}

	if n, ok := req.Find(AVPServingNode, tgpp.VendorID); ok {
		if a.ServingNode, err = parseServingNode(n); err != nil {
			return Alert{}, tgpp.InvalidAVP(n)
		}
	}

	return a, nil
}

func ParseAlertServiceCentreAnswer(ans *diameter.Message) error {
	return parseResult(ans)
}
