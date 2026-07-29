// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

func cause5GMMToString(cause fgs.GMMCause) string {
	switch cause {
	case 0x03:
		return "Illegal UE"
	case 0x05:
		return "PEI Not Accepted"
	case 0x06:
		return "Illegal ME"
	case 0x07:
		return "5GS Services Not Allowed"
	case 0x09:
		return "UE Identity Cannot Be Derived By The Network"
	case 0x0a:
		return "Implicitly Deregistered"
	case 0x0b:
		return "PLMN Not Allowed"
	case 0x0c:
		return "Tracking Area Not Allowed"
	case 0x0d:
		return "Roaming Not Allowed In This Tracking Area"
	case 0x0f:
		return "No Suitable Cells In Tracking Area"
	case 0x14:
		return "MAC Failure"
	case 0x15:
		return "Synch Failure"
	case 0x16:
		return "Congestion"
	case 0x17:
		return "UE Security Capabilities Mismatch"
	case 0x18:
		return "Security Mode Rejected Unspecified"
	case 0x1a:
		return "Non-5G Authentication Unacceptable"
	case 0x1b:
		return "N1 Mode Not Allowed"
	case 0x1c:
		return "Restricted Service Area"
	case 0x2b:
		return "LADN Not Available"
	case 0x41:
		return "Maximum Number Of PDU Sessions Reached"
	case 0x43:
		return "Insufficient Resources For Specific Slice And DNN"
	case 0x45:
		return "Insufficient Resources For Specific Slice"
	case 0x47:
		return "ngKSI Already In Use"
	case 0x48:
		return "Non-3GPP Access To 5GCN Not Allowed"
	case 0x49:
		return "Serving Network Not Authorized"
	case 0x5a:
		return "Payload Was Not Forwarded"
	case 0x5b:
		return "DNN Not Supported Or Not Subscribed In The Slice"
	case 0x5c:
		return "Insufficient User Plane Resources For The PDU Session"
	case 0x5f:
		return "Semantically Incorrect Message"
	case 0x60:
		return "Invalid Mandatory Information"
	case 0x61:
		return "Message Type Non Existent Or Not Implemented"
	case 0x62:
		return "Message Type Not Compatible With The Protocol State"
	case 0x63:
		return "Information Element Non Existent Or Not Implemented"
	case 0x64:
		return "Conditional IE Error"
	case 0x65:
		return "Message Not Compatible With The Protocol State"
	case 0x6f:
		return "Protocol Error Unspecified"
	default:
		return fmt.Sprintf("Unknown Cause (%d)", cause)
	}
}

func cause5GSMToString(cause fgs.GSMCause) string {
	switch cause {
	case 0x1a:
		return "Insufficient Resources"
	case 0x1b:
		return "Missing Or Unknown DNN"
	case 0x1c:
		return "Unknown PDU Session Type"
	case 0x1d:
		return "User Authentication Or Authorization Failed"
	case 0x1f:
		return "Request Rejected Unspecified"
	case 0x22:
		return "Service Option Temporarily Out Of Order"
	case 0x23:
		return "PTI Already In Use"
	case 0x24:
		return "Regular Deactivation"
	case 0x26:
		return "Network Failure"
	case 0x27:
		return "Reactivation Requested"
	case 0x2b:
		return "Invalid PDU Session Identity"
	case 0x2c:
		return "Semantic Errors In Packet Filter"
	case 0x2d:
		return "Syntactical Error In Packet Filter"
	case 0x2e:
		return "Out Of LADN Service Area"
	case 0x2f:
		return "PTI Mismatch"
	case 0x32:
		return "PDU Session Type IPv4 Only Allowed"
	case 0x33:
		return "PDU Session Type IPv6 Only Allowed"
	case 0x36:
		return "PDU Session Does Not Exist"
	case 0x43:
		return "Insufficient Resources For Specific Slice And DNN"
	case 0x44:
		return "Not Supported SSC Mode"
	case 0x45:
		return "Insufficient Resources For Specific Slice"
	case 0x46:
		return "Missing Or Unknown DNN In A Slice"
	case 0x51:
		return "Invalid PTI Value"
	case 0x52:
		return "Maximum Data Rate Per UE For User Plane Integrity Protection Is Too Low"
	case 0x53:
		return "Semantic Error In The QoS Operation"
	case 0x54:
		return "Syntactical Error In The QoS Operation"
	case 0x55:
		return "Invalid Mapped EPS Bearer Identity"
	case 0x5f:
		return "Semantically Incorrect Message"
	case 0x60:
		return "Invalid Mandatory Information"
	case 0x61:
		return "Message Type Non Existent Or Not Implemented"
	case 0x62:
		return "Message Type Not Compatible With The Protocol State"
	case 0x63:
		return "Information Element Non Existent Or Not Implemented"
	case 0x64:
		return "Conditional IE Error"
	case 0x65:
		return "Message Not Compatible With The Protocol State"
	case 0x6f:
		return "Protocol Error Unspecified"
	default:
		return fmt.Sprintf("Unknown Cause (%d)", cause)
	}
}
