// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/s1ap"
)

// SONConfigurationTransfer is the SON container the two eNBs exchange through
// the MME. Its content is X2AP, opaque to S1AP (TS 36.413 §9.2.3.26).
type SONConfigurationTransfer struct {
	Hex string `json:"hex"`
}

func buildENBConfigurationTransfer(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseENBConfigurationTransfer(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse eNB Configuration Transfer: %v", err)}, ""
	}

	var ies []IE

	if m.SONConfigurationTransfer != nil {
		ies = append(ies, ie(s1ap.IDSONConfigurationTransferECT, s1ap.CriticalityIgnore,
			sonConfigurationTransfer(m.SONConfigurationTransfer)))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDENDCSONConfigurationTransferECT, s1ap.CriticalityIgnore, m.ENDCSONConfigurationTransferECT},
		{s1ap.IDIntersystemSONConfigurationTransferECT, s1ap.CriticalityIgnore, m.IntersystemSONConfigurationTransferECT},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "eNB Configuration Transfer"
}

func buildMMEConfigurationTransfer(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseMMEConfigurationTransfer(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse MME Configuration Transfer: %v", err)}, ""
	}

	var ies []IE

	if m.SONConfigurationTransfer != nil {
		ies = append(ies, ie(s1ap.IDSONConfigurationTransferMCT, s1ap.CriticalityIgnore,
			sonConfigurationTransfer(m.SONConfigurationTransfer)))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDENDCSONConfigurationTransferMCT, s1ap.CriticalityIgnore, m.ENDCSONConfigurationTransferMCT},
		{s1ap.IDIntersystemSONConfigurationTransferMCT, s1ap.CriticalityIgnore, m.IntersystemSONConfigurationTransferMCT},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "MME Configuration Transfer"
}

func sonConfigurationTransfer(v *s1ap.SONConfigurationTransfer) SONConfigurationTransfer {
	w := per.NewWriter()
	if err := v.MarshalPER(w, per.Aligned); err != nil {
		return SONConfigurationTransfer{}
	}

	w.AlignToByte()

	return SONConfigurationTransfer{Hex: hex.EncodeToString(w.Bytes())}
}
