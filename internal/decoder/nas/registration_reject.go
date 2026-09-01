// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationReject struct {
	Cause5GMM utils.EnumField `json:"cause_5gmm"`

	T3346Value *UnsupportedIE `json:"t3346_value,omitempty"`
	T3502Value *UnsupportedIE `json:"t3502_value,omitempty"`
	EAPMessage *UnsupportedIE `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildRegistrationReject(msg *fgs.RegistrationReject) *RegistrationReject {
	out := &RegistrationReject{
		Cause5GMM: cause5GMMToEnum(msg.Cause),
	}

	if msg.T3346 != nil {
		out.T3346Value = makeUnsupportedIE()
	}

	if msg.T3502 != nil {
		out.T3502Value = makeUnsupportedIE()
	}

	if msg.EAP != nil {
		out.EAPMessage = makeUnsupportedIE()
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
