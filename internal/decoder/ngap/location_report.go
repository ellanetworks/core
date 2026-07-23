// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

func buildLocationReport(locationReport ngapType.LocationReport) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(locationReport.ProtocolIEs.List); i++ {
		ie := locationReport.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDUserLocationInformation:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUserLocationInformationIE(*ie.Value.UserLocationInformation),
			})
		case ngapType.ProtocolIEIDLocationReportingRequestType:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildLocationReportingRequestType(ie.Value.LocationReportingRequestType),
			})
		case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUEPresenceInAreaOfInterestList(*ie.Value.UEPresenceInAreaOfInterestList),
			})
		default:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Error:       fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
			})
		}
	}

	return NGAPMessageValue{
		IEs: ies,
	}
}

type LocationReportingRequestType struct {
	EventType                                 string               `json:"event_type"`
	ReportArea                                string               `json:"report_area"`
	AreaOfInterestList                        []AreaOfInterestItem `json:"area_of_interest_list,omitempty"`
	LocationReportingReferenceIDToBeCancelled *int64               `json:"reference_id_to_be_cancelled,omitempty"`
}

type AreaOfInterestItem struct {
	LocationReportingReferenceID int64 `json:"location_reporting_reference_id"`
	GADIE                        any   `json:"gad_ie"`
}

func buildLocationReportingRequestType(lrrt *ngapType.LocationReportingRequestType) LocationReportingRequestType {
	var out LocationReportingRequestType
	if lrrt == nil {
		return out
	}

	out.EventType = eventTypeToString(lrrt.EventType)
	out.ReportArea = reportAreaToString(lrrt.ReportArea)

	if lrrt.AreaOfInterestList != nil {
		out.AreaOfInterestList = buildAreaOfInterestList(lrrt.AreaOfInterestList)
	}

	if lrrt.LocationReportingReferenceIDToBeCancelled != nil {
		refID := lrrt.LocationReportingReferenceIDToBeCancelled.Value
		out.LocationReportingReferenceIDToBeCancelled = &refID
	}

	return out
}

func buildAreaOfInterestList(aoiList *ngapType.AreaOfInterestList) []AreaOfInterestItem {
	if aoiList == nil {
		return nil
	}

	items := make([]AreaOfInterestItem, 0)

	for i := 0; i < len(aoiList.List); i++ {
		item := aoiList.List[i]
		aoiItem := AreaOfInterestItem{
			LocationReportingReferenceID: item.LocationReportingReferenceID.Value,
		}

		aoi := item.AreaOfInterest
		if aoi.AreaOfInterestTAIList != nil || aoi.AreaOfInterestCellList != nil || aoi.AreaOfInterestRANNodeList != nil {
			aoiItem.GADIE = map[string]any{
				"error": "GAD decoding not implemented",
			}
		} else {
			aoiItem.GADIE = map[string]any{
				"error": "no area of interest defined",
			}
		}

		items = append(items, aoiItem)
	}

	return items
}

type UEPresenceInAreaOfInterestList struct {
	Items []UEPresenceInAreaOfInterestItem `json:"items"`
}

type UEPresenceInAreaOfInterestItem struct {
	LocationReportingReferenceID int64  `json:"location_reporting_reference_id"`
	UEPresence                   string `json:"ue_presence"` // "In", "Out"
}

func buildUEPresenceInAreaOfInterestList(list ngapType.UEPresenceInAreaOfInterestList) UEPresenceInAreaOfInterestList {
	items := make([]UEPresenceInAreaOfInterestItem, 0)

	for i := 0; i < len(list.List); i++ {
		item := list.List[i]
		uePresence := "Unknown"

		switch item.UEPresence.Value {
		case ngapType.UEPresencePresentIn:
			uePresence = "In"
		case ngapType.UEPresencePresentOut:
			uePresence = "Out"
		}

		items = append(items, UEPresenceInAreaOfInterestItem{
			LocationReportingReferenceID: item.LocationReportingReferenceID.Value,
			UEPresence:                   uePresence,
		})
	}

	return UEPresenceInAreaOfInterestList{Items: items}
}

func eventTypeToString(eventType ngapType.EventType) string {
	switch eventType.Value {
	case ngapType.EventTypePresentDirect:
		return "Direct"
	case ngapType.EventTypePresentChangeOfServeCell:
		return "ChangeOfServingCell"
	case ngapType.EventTypePresentUePresenceInAreaOfInterest:
		return "UePresenceInAreaOfInterest"
	case ngapType.EventTypePresentStopChangeOfServeCell:
		return "StopChangeOfServingCell"
	case ngapType.EventTypePresentStopUePresenceInAreaOfInterest:
		return "StopUePresenceInAreaOfInterest"
	case ngapType.EventTypePresentCancelLocationReportingForTheUe:
		return "CancelLocationReportingForTheUe"
	default:
		return fmt.Sprintf("Unknown(%d)", eventType.Value)
	}
}

func reportAreaToString(reportArea ngapType.ReportArea) string {
	switch reportArea.Value {
	case ngapType.ReportAreaPresentCell:
		return "Cell"
	default:
		return fmt.Sprintf("Unknown(%d)", reportArea.Value)
	}
}
