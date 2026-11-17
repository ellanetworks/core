// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

type SourceToTargetTransparentContainer struct {
	Hex                  string                                    `json:"hex"`
	SourceeNBToTargeteNB *SourceeNBToTargeteNBTransparentContainer `json:"source_enb_to_target_enb,omitempty"`
	Error                string                                    `json:"error,omitempty"`
}

type SourceeNBToTargeteNBTransparentContainer struct {
	RRCContainer              string            `json:"rrc_container"`
	ERABInformationList       []ERABInformation `json:"erab_information_list,omitempty"`
	TargetCellID              EUTRANCGI         `json:"target_cell_id"`
	SubscriberProfileIDforRFP *uint16           `json:"subscriber_profile_id_for_rfp,omitempty"`
	UEHistoryInformation      []LastVisitedCell `json:"ue_history_information"`
}

type ERABInformation struct {
	ERABID       uint8            `json:"erab_id"`
	DLForwarding *utils.EnumField `json:"dl_forwarding,omitempty"`
}

type LastVisitedCell struct {
	EUTRANCell *LastVisitedEUTRANCell `json:"e_utran_cell,omitempty"`
	UTRANCell  *string                `json:"utran_cell,omitempty"`
	GERANCell  *string                `json:"geran_cell,omitempty"`
	NGRANCell  *string                `json:"ng_ran_cell,omitempty"`
}

type LastVisitedEUTRANCell struct {
	GlobalCellID       EUTRANCGI       `json:"global_cell_id"`
	CellSize           utils.EnumField `json:"cell_size"`
	TimeUEStayedInCell uint16          `json:"time_ue_stayed_in_cell"`
}

func sourceToTargetTransparentContainer(b []byte, sourceeNB bool) SourceToTargetTransparentContainer {
	out := SourceToTargetTransparentContainer{Hex: hex.EncodeToString(b)}

	if !sourceeNB {
		return out
	}

	c, err := s1ap.DecodeSourceeNBToTargeteNBTransparentContainer(b)
	if err != nil {
		out.Error = err.Error()

		return out
	}

	out.SourceeNBToTargeteNB = sourceeNBToTargeteNB(c)

	return out
}

func sourceeNBToTargeteNB(c *s1ap.SourceeNBToTargeteNBTransparentContainer) *SourceeNBToTargeteNBTransparentContainer {
	out := &SourceeNBToTargeteNBTransparentContainer{
		RRCContainer:         hex.EncodeToString(c.RRCContainer),
		TargetCellID:         eutranCGI(c.TargetCellID),
		UEHistoryInformation: make([]LastVisitedCell, 0, len(c.UEHistoryInformation)),
	}

	for _, it := range c.ERABInformationList {
		e := ERABInformation{ERABID: uint8(it.ERABID)}

		if it.DLForwarding != nil {
			f := namedEnum(uint8(*it.DLForwarding), it.DLForwarding.Name())
			e.DLForwarding = &f
		}

		out.ERABInformationList = append(out.ERABInformationList, e)
	}

	if c.SubscriberProfileIDforRFP != nil {
		v := uint16(*c.SubscriberProfileIDforRFP)
		out.SubscriberProfileIDforRFP = &v
	}

	for _, it := range c.UEHistoryInformation {
		out.UEHistoryInformation = append(out.UEHistoryInformation, lastVisitedCell(it))
	}

	return out
}

func lastVisitedCell(it s1ap.LastVisitedCellItem) LastVisitedCell {
	var out LastVisitedCell

	switch {
	case it.EUTRANCell != nil:
		out.EUTRANCell = &LastVisitedEUTRANCell{
			GlobalCellID:       eutranCGI(it.EUTRANCell.GlobalCellID),
			CellSize:           namedEnum(uint8(it.EUTRANCell.CellType.CellSize), it.EUTRANCell.CellType.CellSize.Name()),
			TimeUEStayedInCell: uint16(it.EUTRANCell.TimeUEStayedInCell),
		}
	case it.UTRANCell != nil:
		v := hex.EncodeToString(*it.UTRANCell)
		out.UTRANCell = &v
	case it.GERANCell != nil:
		v := "undefined"
		out.GERANCell = &v
	case it.NGRANCell != nil:
		v := hex.EncodeToString(*it.NGRANCell)
		out.NGRANCell = &v
	}

	return out
}
