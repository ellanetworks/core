// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type NotifyRequest struct {
	IMSI                      string
	Features                  Features
	Terminal                  *TerminalInformation
	PDNGateway                *MIP6AgentInfo
	VisitedNetworkIdentifier  string
	ContextIdentifier         uint32
	ServiceSelection          string
	AlertReason               *AlertReason
	UESRVCCCapability         *UESRVCCCapability
	Flags                     NORFlags
	HomogeneousIMSVoiceOverPS *IMSVoiceOverPS
	MaximumUEAvailabilityTime time.Time
}

var norRules = requestRules(false, userNameRule.With(diameter.Rules{
	vendorKey(AVPTerminalInformation):                       {},
	{Code: AVPMIP6AgentInfo}:                                {},
	vendorKey(tgpp.AVPVisitedNetworkIdentifier):             {},
	vendorKey(AVPContextIdentifier):                         {},
	{Code: AVPServiceSelection}:                             {},
	vendorKey(AVPAlertReason):                               {},
	vendorKey(AVPUESRVCCCapability):                         {},
	vendorKey(AVPNORFlags):                                  {},
	vendorKey(AVPHomogeneousSupportOfIMSVoiceOverPSSession): {},
	vendorKey(tgpp.AVPMaximumUEAvailabilityTime):            {},
	vendorKey(AVPMonitoringEventConfigStatus):               {Multiple: true},
	vendorKey(AVPEmergencyServices):                         {},
}))

func NewNotifyRequest(env tgpp.Envelope, r NotifyRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)

	if r.Terminal != nil {
		terminal, err := r.Terminal.avp()
		if err != nil {
			return nil, err
		}

		avps = append(avps, terminal)
	}

	if r.PDNGateway != nil {
		gw, err := r.PDNGateway.avp()
		if err != nil {
			return nil, err
		}

		avps = append(avps, gw)
	}

	avps = appendString(avps, tgpp.AVPVisitedNetworkIdentifier, r.VisitedNetworkIdentifier)

	if r.ContextIdentifier != 0 {
		avps = append(avps, unsignedAVP(AVPContextIdentifier, r.ContextIdentifier))
	}

	if r.ServiceSelection != "" {
		avps = append(avps, diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, r.ServiceSelection))
	}

	if avps, err = appendEnum(avps, AVPAlertReason, "Alert-Reason", r.AlertReason); err != nil {
		return nil, err
	}

	if avps, err = appendEnum(avps, AVPUESRVCCCapability, "UE-SRVCC-Capability", r.UESRVCCCapability); err != nil {
		return nil, err
	}

	if avps, err = appendFlags(avps, AVPNORFlags, "NOR-Flags", r.Flags, norFlagsDefined); err != nil {
		return nil, err
	}

	if avps, err = appendEnum(avps, AVPHomogeneousSupportOfIMSVoiceOverPSSession, "Homogeneous-Support-of-IMS-Voice-Over-PS-Sessions", r.HomogeneousIMSVoiceOverPS); err != nil {
		return nil, err
	}

	if !r.MaximumUEAvailabilityTime.IsZero() {
		avps = append(avps, diameter.Time(tgpp.AVPMaximumUEAvailabilityTime, 0, tgpp.VendorID, r.MaximumUEAvailabilityTime))
	}

	return newRequest(env, CommandNotify, false, avps...)
}

func CheckNotify(req *diameter.Message) error {
	return norRules.Check(req)
}

func ParseNotifyRequest(req *diameter.Message) (NotifyRequest, error) {
	if err := CheckNotify(req); err != nil {
		return NotifyRequest{}, err
	}

	var (
		r   NotifyRequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return NotifyRequest{}, err
	}

	r.Features = featuresOf(req)

	if t, ok := req.Find(AVPTerminalInformation, tgpp.VendorID); ok {
		if r.Terminal, err = parseTerminalInformation(t); err != nil {
			return NotifyRequest{}, err
		}
	}

	if gw, ok := req.Find(AVPMIP6AgentInfo, 0); ok {
		if r.PDNGateway, err = parseMIP6AgentInfo(gw); err != nil {
			return NotifyRequest{}, err
		}
	}

	if r.VisitedNetworkIdentifier, err = optionalString(req.AVPs, tgpp.AVPVisitedNetworkIdentifier, tgpp.VendorID); err != nil {
		return NotifyRequest{}, err
	}

	if r.ContextIdentifier, _, err = optionalUnsigned(req.AVPs, AVPContextIdentifier); err != nil {
		return NotifyRequest{}, err
	}

	if r.ServiceSelection, err = optionalString(req.AVPs, AVPServiceSelection, 0); err != nil {
		return NotifyRequest{}, err
	}

	if r.AlertReason, err = optionalEnum[AlertReason](req.AVPs, AVPAlertReason); err != nil {
		return NotifyRequest{}, err
	}

	if r.UESRVCCCapability, err = optionalEnum[UESRVCCCapability](req.AVPs, AVPUESRVCCCapability); err != nil {
		return NotifyRequest{}, err
	}

	if r.Flags, err = optionalFlags(req.AVPs, AVPNORFlags, norFlagsDefined); err != nil {
		return NotifyRequest{}, err
	}

	if r.HomogeneousIMSVoiceOverPS, err = optionalEnum[IMSVoiceOverPS](req.AVPs, AVPHomogeneousSupportOfIMSVoiceOverPSSession); err != nil {
		return NotifyRequest{}, err
	}

	if t, ok := req.Find(tgpp.AVPMaximumUEAvailabilityTime, tgpp.VendorID); ok {
		if r.MaximumUEAvailabilityTime, err = t.Time(); err != nil {
			return NotifyRequest{}, tgpp.InvalidAVP(t)
		}
	}

	return r, nil
}

func NewNotifyAnswer(req *diameter.Message, id diameter.Identity, features Features) *diameter.Message {
	return newSuccessAnswer(req, id, features)
}

func ParseNotifyAnswer(ans *diameter.Message) (Features, error) {
	return parseAnswer(ans)
}
