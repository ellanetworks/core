// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationReject struct {
	Cause5GMM utils.EnumField `json:"cause_5gmm"`

	T3346Value *GPRSTimer2Value `json:"t3346_value,omitempty"`
	T3502Value *GPRSTimer2Value `json:"t3502_value,omitempty"`
	EAPMessage *utils.RawOctets `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildRegistrationReject(msg *fgs.RegistrationReject) *RegistrationReject {
	out := &RegistrationReject{
		Cause5GMM: cause5GMMToEnum(msg.Cause),
	}

	out.EAPMessage = utils.NewRawOctets(msg.EAP)
	out.T3346Value = gprsTimer2(msg.T3346)
	out.T3502Value = gprsTimer2(msg.T3502)

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
