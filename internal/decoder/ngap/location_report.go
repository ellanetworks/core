// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type LocationReportingRequestType struct {
	EventType                                 string `json:"event_type"`
	ReportArea                                string `json:"report_area"`
	LocationReportingReferenceIDToBeCancelled *int64 `json:"reference_id_to_be_cancelled,omitempty"`
}

type UEPresenceInAreaOfInterestItem struct {
	LocationReportingReferenceID int64  `json:"location_reporting_reference_id"`
	UEPresence                   string `json:"ue_presence"`
}

type UEPresenceInAreaOfInterestList struct {
	Items []UEPresenceInAreaOfInterestItem `json:"items"`
}

// Location Report carries where the NG-RAN node currently sees the UE
// (TS 38.413 §9.2.8.3).
func buildLocationReport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseLocationReport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Location Report: %v", err)}
	}

	ies := []IE{
		ie(idAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(idRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(idUserLocationInformation, ngap.CriticalityIgnore,
			userLocationInformation(*m.UserLocationInformation)))
	}

	if m.UEPresenceInAreaOfInterestList != nil {
		items := make([]UEPresenceInAreaOfInterestItem, 0, len(m.UEPresenceInAreaOfInterestList))
		for _, item := range m.UEPresenceInAreaOfInterestList {
			items = append(items, UEPresenceInAreaOfInterestItem{
				LocationReportingReferenceID: int64(item.LocationReportingReferenceID),
				UEPresence:                   uePresenceText(item.UEPresence),
			})
		}

		ies = append(ies, ie(idUEPresenceInAreaOfInterestList, ngap.CriticalityIgnore,
			UEPresenceInAreaOfInterestList{Items: items}))
	}

	if m.LocationReportingRequestType != nil {
		ies = append(ies, ie(idLocationReportingRequestType, ngap.CriticalityIgnore,
			libLocationReportingRequestType(*m.LocationReportingRequestType)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// Location Reporting Control asks the NG-RAN node to start, change or stop
// reporting the UE's location (TS 38.413 §9.2.8.1).
func buildLocationReportingControl(value []byte) NGAPMessageValue {
	m, err := ngap.ParseLocationReportingControl(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Location Reporting Control: %v", err)}
	}

	ies := []IE{
		ie(idAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(idRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.LocationReportingRequestType != nil {
		ies = append(ies, ie(idLocationReportingRequestType, ngap.CriticalityIgnore,
			libLocationReportingRequestType(*m.LocationReportingRequestType)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// The library does not model areaOfInterestList — this core never requests
// area-of-interest reporting — so the field is absent rather than rendered as
// the "not implemented" placeholder the reference decoder emitted.
func libLocationReportingRequestType(t ngap.LocationReportingRequestType) LocationReportingRequestType {
	out := LocationReportingRequestType{
		EventType:  eventTypeText(t.EventType),
		ReportArea: reportAreaText(t.ReportArea),
	}

	if t.LocationReportingReferenceIDToBeCancelled != nil {
		id := int64(*t.LocationReportingReferenceIDToBeCancelled)
		out.LocationReportingReferenceIDToBeCancelled = &id
	}

	return out
}

func eventTypeText(e ngap.EventType) string {
	switch e {
	case ngap.EventTypeDirect:
		return "Direct"
	case ngap.EventTypeChangeOfServeCell:
		return "ChangeOfServingCell"
	case ngap.EventTypeUEPresenceInAreaOfInterest:
		return "UePresenceInAreaOfInterest"
	case ngap.EventTypeStopChangeOfServeCell:
		return "StopChangeOfServingCell"
	case ngap.EventTypeStopUEPresenceInAreaOfInterest:
		return "StopUePresenceInAreaOfInterest"
	case ngap.EventTypeCancelLocationReportingForTheUE:
		return "CancelLocationReportingForTheUe"
	default:
		return fmt.Sprintf("Unknown(%d)", e)
	}
}

func reportAreaText(a ngap.ReportArea) string {
	if a == ngap.ReportAreaCell {
		return "Cell"
	}

	return fmt.Sprintf("Unknown(%d)", a)
}

func uePresenceText(p ngap.UEPresence) string {
	switch p {
	case ngap.UEPresenceIn:
		return "In"
	case ngap.UEPresenceOut:
		return "Out"
	default:
		return "Unknown"
	}
}
