// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type ServiceReject struct {
	Cause5GMM        utils.EnumField       `json:"cause"`
	PDUSessionStatus []PDUSessionStatusPDU `json:"pdu_session_status,omitempty"`
	T3346Value       *uint8                `json:"t3346_value,omitempty"`
	EAPMessage       []byte                `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildServiceReject(msg *fgs.ServiceReject) *ServiceReject {
	out := &ServiceReject{
		Cause5GMM:  cause5GMMToEnum(msg.Cause),
		T3346Value: timerOctetPtr(msg.T3346),
		EAPMessage: msg.EAP,
	}

	if msg.PDUSessionStatus != nil {
		out.PDUSessionStatus = decodePDUSessionStatus(msg.PDUSessionStatus)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// timerOctetPtr narrows an optional GPRS timer to the raw octet the decoder's
// JSON shape carries.
func timerOctetPtr(t *naslib.GPRSTimer2) *uint8 {
	if t == nil {
		return nil
	}

	raw, err := t.MarshalBinary()
	if err != nil || len(raw) == 0 {
		return nil
	}

	return &raw[0]
}
