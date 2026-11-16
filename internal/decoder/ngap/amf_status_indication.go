// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type UnavailableGUAMI struct {
	Guami
	TimerApproachForGUAMIRemoval bool             `json:"timer_approach_for_guami_removal,omitempty"`
	BackupAMFName                *string          `json:"backup_amf_name,omitempty"`
	ExtendedBackupAMFName        *ExtendedAMFName `json:"extended_backup_amf_name,omitempty"`
}

// AMF Status Indication tells the NG-RAN node which GUAMIs this AMF can no
// longer serve (TS 38.413 §8.7.6). TS 36.413 defines no counterpart.
func buildAMFStatusIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFStatusIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Status Indication: %v", err)}
	}

	guamis := make([]UnavailableGUAMI, 0, len(m.UnavailableGUAMIList))
	for _, item := range m.UnavailableGUAMIList {
		entry := UnavailableGUAMI{
			Guami:                        guami(item.GUAMI),
			TimerApproachForGUAMIRemoval: item.TimerApproachForGUAMIRemoval != nil,
		}

		if item.BackupAMFName != nil {
			name := string(*item.BackupAMFName)
			entry.BackupAMFName = &name
		}

		if item.ExtendedBackupAMFName != nil {
			name := extendedAMFName(*item.ExtendedBackupAMFName)
			entry.ExtendedBackupAMFName = &name
		}

		guamis = append(guamis, entry)
	}

	ies := []IE{ie(ngap.IDUnavailableGUAMIList, ngap.CriticalityReject, guamis)}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
