// Copyright 2024 Ella Networks

package core

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.uber.org/zap"
)

func writeLineTabbed(sb *strings.Builder, s string, tab int) {
	sb.WriteString(strings.Repeat("  ", tab))
	sb.WriteString(s)
	sb.WriteString("\n")
}

func printSessionEstablishmentRequest(req *message.SessionEstablishmentRequest) {
	var sb strings.Builder
	sb.WriteString("\n")
	writeLineTabbed(&sb, "Session Establishment Request:", 0)
	for _, pdr := range req.CreatePDR {
		sb.WriteString("  Create")
		displayPdr(&sb, pdr)
	}

	for _, far := range req.CreateFAR {
		sb.WriteString("  Create")
		displayFar(&sb, far)
	}

	for _, qer := range req.CreateQER {
		sb.WriteString("  Create")
		displayQer(&sb, qer)
	}

	for _, urr := range req.CreateURR {
		sb.WriteString("  Create")
		displayUrr(&sb, urr)
	}

	if req.CreateBAR != nil {
		sb.WriteString("  Create")
		displayBar(&sb, req.CreateBAR)
	}
	logger.UpfLog.Debug("session establishment request", zap.String("request", sb.String()))
}

// IE Contents of Create/Update/Remove are mostly the same
func printSessionModificationRequest(req *message.SessionModificationRequest) {
	var sb strings.Builder
	sb.WriteString("\n")
	writeLineTabbed(&sb, "Session Modification Request:", 0)
	for _, pdr := range req.CreatePDR {
		sb.WriteString("  Create")
		displayPdr(&sb, pdr)
	}

	for _, far := range req.CreateFAR {
		sb.WriteString("  Create")
		displayFar(&sb, far)
	}

	for _, qer := range req.CreateQER {
		sb.WriteString("  Create")
		displayQer(&sb, qer)
	}

	for _, urr := range req.CreateURR {
		sb.WriteString("  Create")
		displayUrr(&sb, urr)
	}

	if req.CreateBAR != nil {
		sb.WriteString("  Create")
		displayBar(&sb, req.CreateBAR)
	}

	for _, pdr := range req.UpdatePDR {
		sb.WriteString("  Update")
		displayPdr(&sb, pdr)
	}

	for _, far := range req.UpdateFAR {
		sb.WriteString("  Update")
		displayFar(&sb, far)
	}

	for _, qer := range req.UpdateQER {
		sb.WriteString("  Update")
		displayQer(&sb, qer)
	}

	for _, urr := range req.UpdateURR {
		sb.WriteString("  Update")
		displayUrr(&sb, urr)
	}

	if req.UpdateBAR != nil {
		writeLineTabbed(&sb, "Update BAR:", 1)
		barID, err := req.UpdateBAR.BARID()
		if err == nil {
			writeLineTabbed(&sb, fmt.Sprintf("BAR ID: %d ", barID), 2)
		}
		downlink, err := req.UpdateBAR.DownlinkDataNotificationDelay()
		if err == nil {
			writeLineTabbed(&sb, fmt.Sprintf("Downlink Data Notification Delay: %s ", downlink), 2)
		}
		suggestedBufferingPackets, err := req.UpdateBAR.SuggestedBufferingPacketsCount()
		if err == nil {
			writeLineTabbed(&sb, fmt.Sprintf("Suggested Buffering Packets Count: %d ", suggestedBufferingPackets), 2)
		}
		mtEdtControl, err := req.UpdateBAR.MTEDTControlInformation()
		if err == nil {
			writeLineTabbed(&sb, fmt.Sprintf("MT EDI: %d ", mtEdtControl), 2)
		}
	}

	// log.Println("------ Remove:")
	for _, pdr := range req.RemovePDR {
		sb.WriteString("  Remove")
		displayPdr(&sb, pdr)
	}

	for _, far := range req.RemoveFAR {
		sb.WriteString("  Remove")
		displayFar(&sb, far)
	}

	for _, qer := range req.RemoveQER {
		sb.WriteString("  Remove")
		displayQer(&sb, qer)
	}

	for _, urr := range req.RemoveURR {
		sb.WriteString("  Remove")
		displayUrr(&sb, urr)
	}

	if req.RemoveBAR != nil {
		writeLineTabbed(&sb, "Remove BAR:", 1)
		barID, err := req.RemoveBAR.BARID()
		if err == nil {
			writeLineTabbed(&sb, fmt.Sprintf("BAR ID: %d ", barID), 2)
		}
	}
	logger.UpfLog.Debug("session modification request", zap.String("request", sb.String()))
}

func printSessionDeleteRequest(req *message.SessionDeletionRequest) {
	var sb strings.Builder
	sb.WriteString("\n")
	writeLineTabbed(&sb, "Session Deletion Request:", 0)
	writeLineTabbed(&sb, fmt.Sprintf("SEID: %d", req.SEID()), 1)
}

func displayBar(sb *strings.Builder, bar *ie.IE) {
	barID, _ := bar.BARID()
	sb.WriteString(fmt.Sprintf("BAR ID: %d\n", barID))

	downlink, err := bar.DownlinkDataNotificationDelay()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Downlink Data Notification Delay: %s ", downlink), 2)
	}
	suggestedBufferingPackets, err := bar.SuggestedBufferingPacketsCount()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Suggested Buffering Packets Count: %d ", suggestedBufferingPackets), 2)
	}
	mtEdtControl, err := bar.MTEDTControlInformation()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("MT EDI: %d ", mtEdtControl), 2)
	}
}

func displayUrr(sb *strings.Builder, urr *ie.IE) {
	urrID, _ := urr.URRID()
	sb.WriteString(fmt.Sprintf("URR ID: %d \n", urrID))

	measurementMethod, err := urr.MeasurementMethod()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Measurement Method: %d ", measurementMethod), 2)
	}
	volumeThreshold, err := urr.VolumeThreshold()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Volume Threshold: %+v ", volumeThreshold), 2)
	}
	timeThreshold, err := urr.TimeThreshold()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Time Threshold: %d ", timeThreshold), 2)
	}
	monitoringTime, err := urr.MonitoringTime()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Monitoring Time: %s ", monitoringTime.Format(time.RFC3339)), 2)
	}
}

func displayQer(sb *strings.Builder, qer *ie.IE) {
	qerID, _ := qer.QERID()
	sb.WriteString(fmt.Sprintf("QER ID: %d \n", qerID))

	gateStatusDL, err := qer.GateStatusDL()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Gate Status DL: %d ", gateStatusDL), 2)
	}
	gateStatusUL, err := qer.GateStatusUL()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Gate Status UL: %d ", gateStatusUL), 2)
	}
	maxBitrateDL, err := qer.MBRDL()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Max Bitrate DL: %d ", uint32(maxBitrateDL)), 2)
	}
	maxBitrateUL, err := qer.MBRUL()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Max Bitrate UL: %d ", uint32(maxBitrateUL)), 2)
	}
	qfi, err := qer.QFI()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("QFI: %d ", qfi), 2)
	}
}

func displayFar(sb *strings.Builder, far *ie.IE) {
	farID, _ := far.FARID()
	sb.WriteString(fmt.Sprintf("FAR ID: %d \n", farID))

	applyAction, err := far.ApplyAction()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Apply Action: %+v ", applyAction), 2)
	}
	if forwardingParameters, err := far.ForwardingParameters(); err == nil {
		writeLineTabbed(sb, "Forwarding Parameters:", 2)
		for _, forwardingParameter := range forwardingParameters {
			networkInstance, err := forwardingParameter.NetworkInstance()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Network Instance: %s ", networkInstance), 3)
			}
			outerHeaderCreation, err := forwardingParameter.OuterHeaderCreation()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Outer Header Creation: %+v ", outerHeaderCreation), 3)
			}
			redirectInformation, err := forwardingParameter.RedirectInformation()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Redirect Information, server address: %s ", redirectInformation.RedirectServerAddress), 3)
				writeLineTabbed(sb, fmt.Sprintf("Redirect Information, other server address: %s ", redirectInformation.OtherRedirectServerAddress), 3)
			}
			headerEnrichment, err := forwardingParameter.HeaderEnrichment()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Header Enrichment: %s : %s ", headerEnrichment.HeaderFieldName, headerEnrichment.HeaderFieldValue), 3)
			}
		}
	}
	if updateForwardingParameters, err := far.UpdateForwardingParameters(); err == nil {
		writeLineTabbed(sb, "Update forwarding Parameters:", 2)
		for _, updateForwardingParameter := range updateForwardingParameters {
			networkInstance, err := updateForwardingParameter.NetworkInstance()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Network Instance: %s ", networkInstance), 3)
			}
			outerHeaderCreation, err := updateForwardingParameter.OuterHeaderCreation()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Outer Header Creation: %+v ", outerHeaderCreation), 3)
			}
			redirectInformation, err := updateForwardingParameter.RedirectInformation()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Redirect Information, server address: %s ", redirectInformation.RedirectServerAddress), 3)
				writeLineTabbed(sb, fmt.Sprintf("Redirect Information, other server address: %s ", redirectInformation.OtherRedirectServerAddress), 3)
			}
			headerEnrichment, err := updateForwardingParameter.HeaderEnrichment()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("Header Enrichment: %s : %s ", headerEnrichment.HeaderFieldName, headerEnrichment.HeaderFieldValue), 3)
			}
		}
	}

	duplicatingParameters, err := far.DuplicatingParameters()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Duplicating Parameters: %+v ", duplicatingParameters), 2)
	}
	barID, err := far.BARID()
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("BAR ID: %d ", barID), 2)
	}
	transportLevelMarking, err := GetTransportLevelMarking(far)
	if err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Transport Level Marking: %d", transportLevelMarking), 2)
		// DSCP (first octet) and ToS or Traffic Class mask (second octet)
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, transportLevelMarking)
		writeLineTabbed(sb, fmt.Sprintf("DSCP: %x", buf[0]), 3)
		writeLineTabbed(sb, fmt.Sprintf("ToS or Traffic Class mask: %x", buf[1]), 3)
	}
}

func displayPdr(sb *strings.Builder, pdr *ie.IE) {
	pdrID, _ := pdr.PDRID()
	sb.WriteString(fmt.Sprintf("PDR ID: %d \n", pdrID))

	if outerHeaderRemoval, err := pdr.OuterHeaderRemovalDescription(); err == nil {
		writeLineTabbed(sb, fmt.Sprintf("Outer Header Removal: %d ", outerHeaderRemoval), 2)
	}

	if farid, err := pdr.FARID(); err == nil {
		writeLineTabbed(sb, fmt.Sprintf("FAR ID: %d ", farid), 2)
	}

	// No method to get several IEs in go-pfcp. So go through all child IEs
	for _, x := range pdr.ChildIEs {
		if x.Type == ie.QERID {
			qerid, err := x.QERID()
			if err == nil {
				writeLineTabbed(sb, fmt.Sprintf("QER ID: %d ", qerid), 2)
			}
		}
	}

	if urrid, err := pdr.URRID(); err == nil {
		writeLineTabbed(sb, fmt.Sprintf("URR ID: %d ", urrid), 2)
	}

	if barid, err := pdr.BARID(); err == nil {
		writeLineTabbed(sb, fmt.Sprintf("BAR ID: %d ", barid), 2)
	}

	if pdi, err := pdr.PDI(); err == nil {
		srcIfacePdiID := findIEindex(pdi, 20) // IE Type source interface
		srcInterface, _ := pdi[srcIfacePdiID].SourceInterface()
		writeLineTabbed(sb, fmt.Sprintf("Source Interface: %d ", srcInterface), 2)

		if teidPdiID := findIEindex(pdi, 21); teidPdiID != -1 { // IE Type F-TEID
			if fteid, err := pdi[teidPdiID].FTEID(); err == nil {
				writeLineTabbed(sb, fmt.Sprintf("TEID: %d ", fteid.TEID), 2)
				writeLineTabbed(sb, fmt.Sprintf("Ipv4: %+v ", fteid.IPv4Address), 2)
				writeLineTabbed(sb, fmt.Sprintf("Ipv6: %+v ", fteid.IPv6Address), 2)
			}
		}

		if ueipPdiID := findIEindex(pdi, 93); ueipPdiID != -1 { // IE Type UE IP Address
			if ueIP, _ := pdi[ueipPdiID].UEIPAddress(); ueIP != nil {
				if ueIP.IPv4Address != nil {
					writeLineTabbed(sb, fmt.Sprintf("UE IPv4 Address: %s ", ueIP.IPv4Address), 2)
				}
				if ueIP.IPv6Address != nil {
					writeLineTabbed(sb, fmt.Sprintf("UE IPv6 Address: %s ", ueIP.IPv6Address), 2)
				}
			} else {
				logger.UpfLog.Debug("ueIP is nil", zap.Int("ueipPdiID", ueipPdiID))
			}
		}

		if sdfFilterID := findIEindex(pdi, 23); sdfFilterID != -1 { // IE Type SDF Filter
			if sdfFilter, err := pdi[sdfFilterID].SDFFilter(); err == nil {
				writeLineTabbed(sb, fmt.Sprintf("SDF Filter: %s ", sdfFilter.FlowDescription), 2)
			}
		}
	}
}
