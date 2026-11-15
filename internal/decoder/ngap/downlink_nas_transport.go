// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// Downlink NAS Transport relays a NAS message to the UE (TS 38.413 §9.2.5.1).
func buildDownlinkNASTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseDownlinkNASTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Downlink NAS Transport: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDNASPDU, ngap.CriticalityReject, libNASPDU(m.NASPDU)),
	}

	ies = appendTyped(ies, []typedIE{
		{ngap.IDOldAMF, ngap.CriticalityReject, m.OldAMF},
		{ngap.IDRANPagingPriority, ngap.CriticalityIgnore, m.RANPagingPriority},
		{ngap.IDMobilityRestrictionList, ngap.CriticalityIgnore, m.MobilityRestrictionList},
		{ngap.IDIndexToRFSP, ngap.CriticalityIgnore, m.IndexToRFSP},
		{ngap.IDUEAggregateMaximumBitRate, ngap.CriticalityIgnore, m.UEAggregateMaximumBitRate},
		{ngap.IDAllowedNSSAI, ngap.CriticalityReject, m.AllowedNSSAI},
		{ngap.IDSRVCCOperationPossible, ngap.CriticalityIgnore, m.SRVCCOperationPossible},
		{ngap.IDEnhancedCoverageRestriction, ngap.CriticalityIgnore, m.EnhancedCoverageRestriction},
		{ngap.IDExtendedConnectedTime, ngap.CriticalityIgnore, m.ExtendedConnectedTime},
		{ngap.IDUEDifferentiationInfo, ngap.CriticalityIgnore, m.UEDifferentiationInfo},
		{ngap.IDCEmodeBrestricted, ngap.CriticalityIgnore, m.CEmodeBrestricted},
		{ngap.IDUERadioCapability, ngap.CriticalityIgnore, m.UERadioCapability},
		{ngap.IDUECapabilityInfoRequest, ngap.CriticalityIgnore, m.UECapabilityInfoRequest},
		{ngap.IDEndIndication, ngap.CriticalityIgnore, m.EndIndication},
		{ngap.IDUERadioCapabilityID, ngap.CriticalityReject, m.UERadioCapabilityID},
		{ngap.IDTargetNSSAIInformation, ngap.CriticalityIgnore, m.TargetNSSAIInformation},
		{ngap.IDMaskedIMEISV, ngap.CriticalityIgnore, m.MaskedIMEISV},
		{ngap.IDPartiallyAllowedNSSAI, ngap.CriticalityIgnore, m.PartiallyAllowedNSSAI},
		{ngap.IDMobileIABAuthorized, ngap.CriticalityIgnore, m.MobileIABAuthorized},
		{ngap.IDExtendedOldAMF, ngap.CriticalityIgnore, m.ExtendedOldAMF},
	})

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
