// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

const ntpToUnixOffset = 2208988800 // seconds between 1900-01-01 and 1970-01-01

type IE struct {
	ID          utils.EnumField `json:"id"`
	Criticality utils.EnumField `json:"criticality"`
	Value       any             `json:"value,omitempty"`
	ValueType   string          `json:"value_type,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

func inferValueType(v any) string {
	if v == nil {
		return ""
	}

	switch v.(type) {
	case int64, int32, int, uint64, uint32:
		return "integer"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case NASPDU:
		return "nas_pdu"
	case NRPPaPDU:
		return "nrppa_pdu"
	case rawIEValue:
		return "unmodeled"
	case utils.EnumField:
		return "enum"
	default:
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			return "array"
		}

		return "object"
	}
}

func setIEValueTypes(ies []IE) {
	for i := range ies {
		ies[i].ValueType = inferValueType(ies[i].Value)
	}
}

func criticalityToEnum(c ngap.Criticality) utils.EnumField {
	switch c {
	case ngap.CriticalityReject:
		return utils.MakeEnum(uint64(c), "Reject", false)
	case ngap.CriticalityIgnore:
		return utils.MakeEnum(uint64(c), "Ignore", false)
	case ngap.CriticalityNotify:
		return utils.MakeEnum(uint64(c), "Notify", false)
	default:
		return utils.MakeEnum(uint64(c), "", true)
	}
}

func timeStampToRFC3339(timeStampNgap []byte) (string, error) {
	if len(timeStampNgap) != 4 {
		return "", fmt.Errorf("invalid NGAP timestamp length: got %d, want 4", len(timeStampNgap))
	}

	ntpSeconds := binary.BigEndian.Uint32(timeStampNgap)
	unixSeconds := int64(ntpSeconds) - ntpToUnixOffset
	t := time.Unix(unixSeconds, 0).UTC()

	return t.Format(time.RFC3339), nil
}

// ieNames is the label vocabulary the events drawer renders IE ids with,
// keyed on the wire value (TS 38.413 §9.4.5). internal/decoder/s1ap keeps the
// same shape.
var ieNames = map[int64]string{
	0:   "AllowedNSSAI",
	1:   "AMFName",
	2:   "AMFOverloadResponse",
	3:   "AMFSetID",
	4:   "AMFTNLAssociationFailedToSetupList",
	5:   "AMFTNLAssociationSetupList",
	6:   "AMFTNLAssociationToAddList",
	7:   "AMFTNLAssociationToRemoveList",
	8:   "AMFTNLAssociationToUpdateList",
	9:   "AMFTrafficLoadReductionIndication",
	10:  "AMFUENGAPID",
	11:  "AssistanceDataForPaging",
	12:  "BroadcastCancelledAreaList",
	13:  "BroadcastCompletedAreaList",
	14:  "CancelAllWarningMessages",
	15:  "Cause",
	16:  "CellIDListForRestart",
	17:  "ConcurrentWarningMessageInd",
	18:  "CoreNetworkAssistanceInformation",
	19:  "CriticalityDiagnostics",
	20:  "DataCodingScheme",
	21:  "DefaultPagingDRX",
	22:  "DirectForwardingPathAvailability",
	23:  "EmergencyAreaIDListForRestart",
	24:  "EmergencyFallbackIndicator",
	25:  "EUTRACGI",
	26:  "FiveGSTMSI",
	27:  "GlobalRANNodeID",
	28:  "GUAMI",
	29:  "HandoverType",
	30:  "IMSVoiceSupportIndicator",
	31:  "IndexToRFSP",
	32:  "InfoOnRecommendedCellsAndRANNodesForPaging",
	33:  "LocationReportingRequestType",
	34:  "MaskedIMEISV",
	35:  "MessageIdentifier",
	36:  "MobilityRestrictionList",
	37:  "NASC",
	38:  "NASPDU",
	39:  "NASSecurityParametersFromNGRAN",
	40:  "NewAMFUENGAPID",
	41:  "NewSecurityContextInd",
	42:  "NGAPMessage",
	43:  "NGRANCGI",
	44:  "NGRANTraceID",
	45:  "NRCGI",
	46:  "NRPPaPDU",
	47:  "NumberOfBroadcastsRequested",
	48:  "OldAMF",
	49:  "OverloadStartNSSAIList",
	50:  "PagingDRX",
	51:  "PagingOrigin",
	52:  "PagingPriority",
	53:  "PDUSessionResourceAdmittedList",
	54:  "PDUSessionResourceFailedToModifyListModRes",
	55:  "PDUSessionResourceFailedToSetupListCxtRes",
	56:  "PDUSessionResourceFailedToSetupListHOAck",
	57:  "PDUSessionResourceFailedToSetupListPSReq",
	58:  "PDUSessionResourceFailedToSetupListSURes",
	59:  "PDUSessionResourceHandoverList",
	60:  "PDUSessionResourceListCxtRelCpl",
	61:  "PDUSessionResourceListHORqd",
	62:  "PDUSessionResourceModifyListModCfm",
	63:  "PDUSessionResourceModifyListModInd",
	64:  "PDUSessionResourceModifyListModReq",
	65:  "PDUSessionResourceModifyListModRes",
	66:  "PDUSessionResourceNotifyList",
	67:  "PDUSessionResourceReleasedListNot",
	68:  "PDUSessionResourceReleasedListPSAck",
	69:  "PDUSessionResourceReleasedListPSFail",
	70:  "PDUSessionResourceReleasedListRelRes",
	71:  "PDUSessionResourceSetupListCxtReq",
	72:  "PDUSessionResourceSetupListCxtRes",
	73:  "PDUSessionResourceSetupListHOReq",
	74:  "PDUSessionResourceSetupListSUReq",
	75:  "PDUSessionResourceSetupListSURes",
	76:  "PDUSessionResourceToBeSwitchedDLList",
	77:  "PDUSessionResourceSwitchedList",
	78:  "PDUSessionResourceToReleaseListHOCmd",
	79:  "PDUSessionResourceToReleaseListRelCmd",
	80:  "PLMNSupportList",
	81:  "PWSFailedCellIDList",
	82:  "RANNodeName",
	83:  "RANPagingPriority",
	84:  "RANStatusTransferTransparentContainer",
	85:  "RANUENGAPID",
	86:  "RelativeAMFCapacity",
	87:  "RepetitionPeriod",
	88:  "ResetType",
	89:  "RoutingID",
	90:  "RRCEstablishmentCause",
	91:  "RRCInactiveTransitionReportRequest",
	92:  "RRCState",
	93:  "SecurityContext",
	94:  "SecurityKey",
	95:  "SerialNumber",
	96:  "ServedGUAMIList",
	97:  "SliceSupportList",
	98:  "SONConfigurationTransferDL",
	99:  "SONConfigurationTransferUL",
	100: "SourceAMFUENGAPID",
	101: "SourceToTargetTransparentContainer",
	102: "SupportedTAList",
	103: "TAIListForPaging",
	104: "TAIListForRestart",
	105: "TargetID",
	106: "TargetToSourceTransparentContainer",
	107: "TimeToWait",
	108: "TraceActivation",
	109: "TraceCollectionEntityIPAddress",
	110: "UEAggregateMaximumBitRate",
	111: "UEAssociatedLogicalNGConnectionList",
	112: "UEContextRequest",
	114: "UENGAPIDs",
	115: "UEPagingIdentity",
	116: "UEPresenceInAreaOfInterestList",
	117: "UERadioCapability",
	118: "UERadioCapabilityForPaging",
	119: "UESecurityCapabilities",
	120: "UnavailableGUAMIList",
	121: "UserLocationInformation",
	122: "WarningAreaList",
	123: "WarningMessageContents",
	124: "WarningSecurityInfo",
	125: "WarningType",
	126: "AdditionalULNGUUPTNLInformation",
	127: "DataForwardingNotPossible",
	128: "DLNGUUPTNLInformation",
	129: "NetworkInstance",
	130: "PDUSessionAggregateMaximumBitRate",
	131: "PDUSessionResourceFailedToModifyListModCfm",
	132: "PDUSessionResourceFailedToSetupListCxtFail",
	133: "PDUSessionResourceListCxtRelReq",
	134: "PDUSessionType",
	135: "QosFlowAddOrModifyRequestList",
	136: "QosFlowSetupRequestList",
	137: "QosFlowToReleaseList",
	138: "SecurityIndication",
	139: "ULNGUUPTNLInformation",
	140: "ULNGUUPTNLModifyList",
	141: "WarningAreaCoordinates",
	142: "PDUSessionResourceSecondaryRATUsageList",
	143: "HandoverFlag",
	144: "SecondaryRATUsageInformation",
	145: "PDUSessionResourceReleaseResponseTransfer",
	146: "RedirectionVoiceFallback",
	147: "UERetentionInformation",
	148: "SNSSAI",
	149: "PSCellInformation",
	150: "LastEUTRANPLMNIdentity",
	151: "MaximumIntegrityProtectedDataRateDL",
	152: "AdditionalDLForwardingUPTNLInformation",
	153: "AdditionalDLUPTNLInformationForHOList",
	154: "AdditionalNGUUPTNLInformation",
	155: "AdditionalDLQosFlowPerTNLInformation",
	156: "SecurityResult",
	157: "ENDCSONConfigurationTransferDL",
	158: "ENDCSONConfigurationTransferUL",
}

func ieEnum(id int64) utils.EnumField {
	name, ok := ieNames[id]

	return utils.MakeEnum(id, name, !ok)
}

// transportLayerAddressToString formats a NGAP TransportLayerAddress byte slice
// as a human-readable string per 3GPP TS 38.414 Section 5.1:
//   - 4 bytes  → IPv4 dotted-decimal
//   - 16 bytes → IPv6 colon-notation
//   - 20 bytes → "IPv4+IPv6" dual-stack
func transportLayerAddressToString(addr []byte) string {
	switch len(addr) {
	case 4:
		return fmt.Sprintf("%d.%d.%d.%d", addr[0], addr[1], addr[2], addr[3])
	case 16:
		return net.IP(addr).String()
	case 20:
		return fmt.Sprintf("%s+%s", net.IP(addr[0:4]).String(), net.IP(addr[4:20]).String())
	default:
		return ""
	}
}

// procedureNames is the label vocabulary the events drawer renders procedure
// codes with, in wire order. internal/decoder/s1ap keeps the same shape.
var procedureNames = map[ngap.ProcedureCode]string{
	ngap.ProcAMFConfigurationUpdate:                "AMFConfigurationUpdate",
	ngap.ProcAMFStatusIndication:                   "AMFStatusIndication",
	ngap.ProcCellTrafficTrace:                      "CellTrafficTrace",
	ngap.ProcDeactivateTrace:                       "DeactivateTrace",
	ngap.ProcDownlinkNASTransport:                  "DownlinkNASTransport",
	ngap.ProcDownlinkNonUEAssociatedNRPPaTransport: "DownlinkNonUEAssociatedNRPPaTransport",
	ngap.ProcDownlinkRANConfigurationTransfer:      "DownlinkRANConfigurationTransfer",
	ngap.ProcDownlinkRANStatusTransfer:             "DownlinkRANStatusTransfer",
	ngap.ProcDownlinkUEAssociatedNRPPaTransport:    "DownlinkUEAssociatedNRPPaTransport",
	ngap.ProcErrorIndication:                       "ErrorIndication",
	ngap.ProcHandoverCancel:                        "HandoverCancel",
	ngap.ProcHandoverNotification:                  "HandoverNotification",
	ngap.ProcHandoverPreparation:                   "HandoverPreparation",
	ngap.ProcHandoverResourceAllocation:            "HandoverResourceAllocation",
	ngap.ProcInitialContextSetup:                   "InitialContextSetup",
	ngap.ProcInitialUEMessage:                      "InitialUEMessage",
	ngap.ProcLocationReportingControl:              "LocationReportingControl",
	ngap.ProcLocationReportingFailureIndication:    "LocationReportingFailureIndication",
	ngap.ProcLocationReport:                        "LocationReport",
	ngap.ProcNASNonDeliveryIndication:              "NASNonDeliveryIndication",
	ngap.ProcNGReset:                               "NGReset",
	ngap.ProcNGSetup:                               "NGSetup",
	ngap.ProcOverloadStart:                         "OverloadStart",
	ngap.ProcOverloadStop:                          "OverloadStop",
	ngap.ProcPaging:                                "Paging",
	ngap.ProcPathSwitchRequest:                     "PathSwitchRequest",
	ngap.ProcPDUSessionResourceModify:              "PDUSessionResourceModify",
	ngap.ProcPDUSessionResourceModifyIndication:    "PDUSessionResourceModifyIndication",
	ngap.ProcPDUSessionResourceRelease:             "PDUSessionResourceRelease",
	ngap.ProcPDUSessionResourceSetup:               "PDUSessionResourceSetup",
	ngap.ProcPDUSessionResourceNotify:              "PDUSessionResourceNotify",
	ngap.ProcPrivateMessage:                        "PrivateMessage",
	ngap.ProcPWSCancel:                             "PWSCancel",
	ngap.ProcPWSFailureIndication:                  "PWSFailureIndication",
	ngap.ProcPWSRestartIndication:                  "PWSRestartIndication",
	ngap.ProcRANConfigurationUpdate:                "RANConfigurationUpdate",
	ngap.ProcRerouteNASRequest:                     "RerouteNASRequest",
	ngap.ProcRRCInactiveTransitionReport:           "RRCInactiveTransitionReport",
	ngap.ProcTraceFailureIndication:                "TraceFailureIndication",
	ngap.ProcTraceStart:                            "TraceStart",
	ngap.ProcUEContextModification:                 "UEContextModification",
	ngap.ProcUEContextRelease:                      "UEContextRelease",
	ngap.ProcUEContextReleaseRequest:               "UEContextReleaseRequest",
	ngap.ProcUERadioCapabilityCheck:                "UERadioCapabilityCheck",
	ngap.ProcUERadioCapabilityInfoIndication:       "UERadioCapabilityInfoIndication",
	ngap.ProcUETNLABindingRelease:                  "UETNLABindingRelease",
	ngap.ProcUplinkNASTransport:                    "UplinkNASTransport",
	ngap.ProcUplinkNonUEAssociatedNRPPaTransport:   "UplinkNonUEAssociatedNRPPaTransport",
	ngap.ProcUplinkRANConfigurationTransfer:        "UplinkRANConfigurationTransfer",
	ngap.ProcUplinkRANStatusTransfer:               "UplinkRANStatusTransfer",
	ngap.ProcUplinkUEAssociatedNRPPaTransport:      "UplinkUEAssociatedNRPPaTransport",
	ngap.ProcWriteReplaceWarning:                   "WriteReplaceWarning",
	ngap.ProcSecondaryRATDataUsageReport:           "SecondaryRATDataUsageReport",
	ngap.ProcUplinkRIMInformationTransfer:          "UplinkRIMInformationTransfer",
	ngap.ProcDownlinkRIMInformationTransfer:        "DownlinkRIMInformationTransfer",
	ngap.ProcRetrieveUEInformation:                 "RetrieveUEInformation",
	ngap.ProcUEInformationTransfer:                 "UEInformationTransfer",
	ngap.ProcRANCPRelocationIndication:             "RANCPRelocationIndication",
	ngap.ProcUEContextResume:                       "UEContextResume",
	ngap.ProcUEContextSuspend:                      "UEContextSuspend",
	ngap.ProcUERadioCapabilityIDMapping:            "UERadioCapabilityIDMapping",
	ngap.ProcHandoverSuccess:                       "HandoverSuccess",
	ngap.ProcUplinkRANEarlyStatusTransfer:          "UplinkRANEarlyStatusTransfer",
	ngap.ProcDownlinkRANEarlyStatusTransfer:        "DownlinkRANEarlyStatusTransfer",
	ngap.ProcAMFCPRelocationIndication:             "AMFCPRelocationIndication",
	ngap.ProcConnectionEstablishmentIndication:     "ConnectionEstablishmentIndication",
	ngap.ProcBroadcastSessionModification:          "BroadcastSessionModification",
	ngap.ProcBroadcastSessionRelease:               "BroadcastSessionRelease",
	ngap.ProcBroadcastSessionSetup:                 "BroadcastSessionSetup",
	ngap.ProcDistributionSetup:                     "DistributionSetup",
	ngap.ProcDistributionRelease:                   "DistributionRelease",
	ngap.ProcMulticastSessionActivation:            "MulticastSessionActivation",
	ngap.ProcMulticastSessionDeactivation:          "MulticastSessionDeactivation",
	ngap.ProcMulticastSessionUpdate:                "MulticastSessionUpdate",
	ngap.ProcMulticastGroupPaging:                  "MulticastGroupPaging",
	ngap.ProcBroadcastSessionReleaseRequired:       "BroadcastSessionReleaseRequired",
	ngap.ProcTimingSynchronisationStatus:           "TimingSynchronisationStatus",
	ngap.ProcTimingSynchronisationStatusReport:     "TimingSynchronisationStatusReport",
	ngap.ProcMTCommunicationHandling:               "MTCommunicationHandling",
	ngap.ProcRANPagingRequest:                      "RANPagingRequest",
	ngap.ProcBroadcastSessionTransport:             "BroadcastSessionTransport",
}

func procedureCodeName(code ngap.ProcedureCode) string {
	if name, ok := procedureNames[code]; ok {
		return name
	}

	return ""
}

func procedureCodeToEnum(code ngap.ProcedureCode) utils.EnumField {
	name := procedureCodeName(code)

	return utils.MakeEnum(int64(code), name, name == "")
}

// CriticalityDiagnostics is the decoded CriticalityDiagnostics IE (TS 38.413
// §9.3.1.3): which procedure/message triggered the diagnostic and the offending
// IEs. Absent sub-fields are omitted.
type CriticalityDiagnostics struct {
	ProcedureCode        *utils.EnumField           `json:"procedure_code,omitempty"`
	TriggeringMessage    *utils.EnumField           `json:"triggering_message,omitempty"`
	ProcedureCriticality *utils.EnumField           `json:"procedure_criticality,omitempty"`
	IEs                  []CriticalityDiagnosticsIE `json:"ies,omitempty"`
}

type Guami struct {
	PLMNID      PLMNID `json:"plmn_id"`
	AMFRegionID string `json:"amf_region_id"`
	AMFSetID    string `json:"amf_set_id"`
	AMFPointer  string `json:"amf_pointer"`
}

// CriticalityDiagnosticsIE reports one offending IE (TS 38.413 §9.3.1.3).
type CriticalityDiagnosticsIE struct {
	IEID        utils.EnumField `json:"ie_id"`
	Criticality utils.EnumField `json:"criticality"`
	TypeOfError utils.EnumField `json:"type_of_error"`
}

type PLMN struct {
	PLMNID           PLMNID   `json:"plmn_id"`
	SliceSupportList []SNSSAI `json:"slice_support_list,omitempty"`
}

// ranNodeIDHex renders a RAN node identifier as the hex digits its bit length
// covers, matching how the AMF stores it.

type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

func triggeringMessageToEnum(t ngap.TriggeringMessage) utils.EnumField {
	switch t {
	case ngap.TriggeringInitiatingMessage:
		return utils.MakeEnum(uint64(t), "initiating-message", false)
	case ngap.TriggeringSuccessfulOutcome:
		return utils.MakeEnum(uint64(t), "successful-outcome", false)
	case ngap.TriggeringUnsuccessfulOutcome:
		return utils.MakeEnum(uint64(t), "unsuccessful-outcome", false)
	default:
		return utils.MakeEnum(uint64(t), "", true)
	}
}

func typeOfErrorToEnum(t ngap.TypeOfError) utils.EnumField {
	switch t {
	case ngap.TypeOfErrorNotUnderstood:
		return utils.MakeEnum(uint64(t), "not-understood", false)
	case ngap.TypeOfErrorMissing:
		return utils.MakeEnum(uint64(t), "missing", false)
	default:
		return utils.MakeEnum(uint64(t), "", true)
	}
}
