// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

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

	if len(m.SONConfigurationTransfer) > 0 {
		ies = append(ies, ie(s1ap.IDSONConfigurationTransferECT, s1ap.CriticalityIgnore,
			SONConfigurationTransfer{Hex: hex.EncodeToString(m.SONConfigurationTransfer)}))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "eNB Configuration Transfer"
}

func buildMMEConfigurationTransfer(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseMMEConfigurationTransfer(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse MME Configuration Transfer: %v", err)}, ""
	}

	var ies []IE

	if len(m.SONConfigurationTransfer) > 0 {
		ies = append(ies, ie(s1ap.IDSONConfigurationTransferMCT, s1ap.CriticalityIgnore,
			SONConfigurationTransfer{Hex: hex.EncodeToString(m.SONConfigurationTransfer)}))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "MME Configuration Transfer"
}
