package message

import (
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

func causeToString(cause *ngapType.Cause) string {
	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return radioCauseToString(cause.RadioNetwork.Value)
	case ngapType.CausePresentTransport:
		return transportCauseToString(cause.Transport.Value)
	case ngapType.CausePresentProtocol:
		return protocolCauseToString(cause.Protocol.Value)
	case ngapType.CausePresentNas:
		return nasCauseToString(cause.Nas.Value)
	case ngapType.CausePresentMisc:
		return miscCauseToString(cause.Misc.Value)
	default:
		return "Unknown Cause"
	}
}

func causePresentToString(causePresent int, causeValue aper.Enumerated) string {
	switch causePresent {
	case ngapType.CausePresentNas:
		return nasCauseToString(causeValue)
	case ngapType.CausePresentRadioNetwork:
		return radioCauseToString(causeValue)
	case ngapType.CausePresentTransport:
		return transportCauseToString(causeValue)
	case ngapType.CausePresentProtocol:
		return protocolCauseToString(causeValue)
	case ngapType.CausePresentMisc:
		return miscCauseToString(causeValue)
	default:
		return "Unknown Cause"
	}
}

func radioCauseToString(value aper.Enumerated) string {
	switch value {
	case 0:
		return "Unspecified"
	case 1:
		return "TxN Reloc Overall Expiry"
	case 2:
		return "Successful Handover"
	case 3:
		return "Release Due To Ngran Generated Reason"
	case 4:
		return "Release Due To 5gc Generated Reason"
	case 5:
		return "Handover Cancelled"
	case 6:
		return "PartialHandover"
	case 7:
		return "Ho Failure In Target 5GC Ngran Node Or Target System"
	case 8:
		return "Ho Target Not Allowed"
	case 9:
		return "Tng Reloc Overall Expiry"
	case 10:
		return "Tng Reloc Prep Expiry"
	case 11:
		return "Cell Not Available"
	case 12:
		return "Unknown Target ID"
	case 13:
		return "No Radio Resources Available In Target Cell"
	case 14:
		return "Unknown Local UENGAP ID"
	case 15:
		return "Inconsistent Remote UENGAP ID"
	case 16:
		return "Handover Desirable For Radio Reason"
	case 17:
		return "Time Critical Handover"
	case 18:
		return "Resource Optimisation Handover"
	case 19:
		return "Reduce Load In Serving Cell"
	case 20:
		return "User Inactivity"
	case 21:
		return "Radio Connection With Ue Lost"
	case 22:
		return "Radio Resources Not Available"
	case 23:
		return "Invalid Qos Combination"
	case 24:
		return "Failure In Radio Interface Procedure"
	case 25:
		return "Interaction With Other Procedure"
	case 26:
		return "Unknown PDU Session ID"
	case 27:
		return "Unknown Qos Flow ID"
	case 28:
		return "Multiple PDU Session ID Instances"
	case 29:
		return "Multiple Qos Flow ID Instances"
	case 30:
		return "Encryption And Or Integrity Protection Algorithms Not Supported"
	case 31:
		return "Ng Intra System Handover Triggered"
	case 32:
		return "Ng Inter System Handover Triggered"
	case 33:
		return "Xn Handover Triggered"
	case 34:
		return "Not Supported 5QI Value"
	case 35:
		return "Ue Context Transfer"
	case 36:
		return "Ims Voice Eps Fallback Or Rat Fallback Triggered"
	case 37:
		return "Ims Voice Eps Fallback Triggered"
	case 38:
		return "Ims Voice Eps Fallback Or Rat Fallback Triggered"
	case 39:
		return "Slice Not Supported"
	case 40:
		return "Ims Voice Eps Fallback Triggered"
	case 41:
		return "Redirection"
	case 42:
		return "Resources Not Available For The Slice"
	case 43:
		return "Ims Voice Eps Fallback Or Rat Fallback Triggered"
	case 44:
		return "Ims Voice Eps Fallback Triggered"
	case 45:
		return "N26 Interface Not Available"
	case 46:
		return "Release Due To Pre Emption"
	default:
		return "Unknown Cause"
	}
}

func transportCauseToString(value aper.Enumerated) string {
	switch value {
	case 0:
		return "Transport Resource Unavailable"
	case 1:
		return "Unspecified"
	default:
		return "Unknown Cause"
	}
}

func miscCauseToString(value aper.Enumerated) string {
	switch value {
	case 0:
		return "Control Processing Overload"
	case 1:
		return "Not Enough User Plane Processing Resources"
	case 2:
		return "Hardware Failure"
	case 3:
		return "Om Intervention"
	case 4:
		return "Unknown PLMN"
	case 5:
		return "Unspecified"
	default:
		return "Unknown Cause"
	}
}

func protocolCauseToString(value aper.Enumerated) string {
	switch value {
	case 0:
		return "Transfer Syntax Error"
	case 1:
		return "Abstract Syntax Error Reject"
	case 2:
		return "Abstract Syntax Error Ignore and Notify"
	case 3:
		return "Message Not Compatible with Receiver State"
	case 4:
		return "Semantic Error"
	case 5:
		return "Abstract Syntax Error Falsely Constructed Message"
	case 6:
		return "Unspecified"
	default:
		return "Unknown Cause"
	}
}

func nasCauseToString(value aper.Enumerated) string {
	switch value {
	case 0:
		return "Normal Release"
	case 1:
		return "Authentication Failure"
	case 2:
		return "Deregister"
	case 3:
		return "Unspecified"
	default:
		return "Unknown Cause"
	}
}
