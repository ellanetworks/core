// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import "errors"

func NewAnswer(req *Message, id Identity, resultCode uint32) *Message {
	ans := newAnswer(req, id)

	if resultCode >= 3000 && resultCode < 4000 {
		ans.Flags |= FlagError
	}

	ans.AVPs = append(ans.AVPs, Unsigned32(AVPResultCode, AVPFlagMandatory, 0, resultCode))
	ans.AVPs = append(ans.AVPs, FindAll(req.AVPs, AVPProxyInfo, 0)...)

	return ans
}

func NewExperimentalAnswer(req *Message, id Identity, vendorID, resultCode uint32) *Message {
	ans := newAnswer(req, id)

	ans.AVPs = append(ans.AVPs, Grouped(AVPExperimentalResult, AVPFlagMandatory, 0,
		Unsigned32(AVPVendorID, AVPFlagMandatory, 0, vendorID),
		Unsigned32(AVPExperimentalResultCode, AVPFlagMandatory, 0, resultCode),
	))
	ans.AVPs = append(ans.AVPs, FindAll(req.AVPs, AVPProxyInfo, 0)...)

	return ans
}

func NewErrorAnswer(req *Message, id Identity, err error) *Message {
	var avpErr *AVPError
	if errors.As(err, &avpErr) {
		ans := NewAnswer(req, id, avpErr.ResultCode)
		ans.AVPs = append(ans.AVPs, FailedAVP(avpErr.AVP))

		return ans
	}

	return NewAnswer(req, id, ResultUnableToComply)
}

func FailedAVP(avps ...AVP) AVP {
	return Grouped(AVPFailedAVP, AVPFlagMandatory, 0, avps...)
}

func newAnswer(req *Message, id Identity) *Message {
	ans := &Message{
		Flags:         req.Flags & FlagProxiable,
		CommandCode:   req.CommandCode,
		ApplicationID: req.ApplicationID,
		HopByHopID:    req.HopByHopID,
		EndToEndID:    req.EndToEndID,
	}

	if sessionID, ok := req.Find(AVPSessionID, 0); ok {
		ans.AVPs = append(ans.AVPs, sessionID)
	}

	ans.AVPs = append(ans.AVPs,
		UTF8String(AVPOriginHost, AVPFlagMandatory, 0, id.OriginHost),
		UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, id.OriginRealm),
	)

	return ans
}
