// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type SourceToTargetTransparentContainer struct {
	Hex                              string                                                `json:"hex"`
	SourceNGRANNodeToTargetNGRANNode *SourceNGRANNodeToTargetNGRANNodeTransparentContainer `json:"source_ngran_node_to_target_ngran_node,omitempty"`
	Error                            string                                                `json:"error,omitempty"`
}

type SourceNGRANNodeToTargetNGRANNodeTransparentContainer struct {
	RRCContainer                      string                          `json:"rrc_container"`
	PDUSessionResourceInformationList []PDUSessionResourceInformation `json:"pdu_session_resource_information_list,omitempty"`
	ERABInformationList               []ERABInformation               `json:"erab_information_list,omitempty"`
	TargetCellID                      NGRANCGI                        `json:"target_cell_id"`
	IndexToRFSP                       *uint16                         `json:"index_to_rfsp,omitempty"`
	UEHistoryInformation              []LastVisitedCell               `json:"ue_history_information"`
}

type PDUSessionResourceInformation struct {
	PDUSessionID              int64                  `json:"pdu_session_id"`
	QosFlowInformationList    []QosFlowInformation   `json:"qos_flow_information_list"`
	DRBsToQosFlowsMappingList []DRBToQosFlowsMapping `json:"drbs_to_qos_flows_mapping_list,omitempty"`
}

type QosFlowInformation struct {
	QosFlowIdentifier int64            `json:"qos_flow_identifier"`
	DLForwarding      *utils.EnumField `json:"dl_forwarding,omitempty"`
}

type DRBToQosFlowsMapping struct {
	DRBID                 int64                     `json:"drb_id"`
	AssociatedQosFlowList []MappedAssociatedQosFlow `json:"associated_qos_flow_list"`
}

type MappedAssociatedQosFlow struct {
	QosFlowIdentifier        int64            `json:"qos_flow_identifier"`
	QosFlowMappingIndication *utils.EnumField `json:"qos_flow_mapping_indication,omitempty"`
}

type ERABInformation struct {
	ERABID       int64            `json:"erab_id"`
	DLForwarding *utils.EnumField `json:"dl_forwarding,omitempty"`
}

type LastVisitedCell struct {
	NGRANCell  *LastVisitedNGRANCell `json:"ng_ran_cell,omitempty"`
	EUTRANCell *string               `json:"e_utran_cell,omitempty"`
	UTRANCell  *string               `json:"utran_cell,omitempty"`
	GERANCell  *string               `json:"geran_cell,omitempty"`
}

type LastVisitedNGRANCell struct {
	GlobalCellID                          NGRANCGI        `json:"global_cell_id"`
	CellSize                              utils.EnumField `json:"cell_size"`
	TimeUEStayedInCell                    uint16          `json:"time_ue_stayed_in_cell"`
	TimeUEStayedInCellEnhancedGranularity *uint16         `json:"time_ue_stayed_in_cell_enhanced_granularity,omitempty"`
	HOCauseValue                          *Cause          `json:"ho_cause_value,omitempty"`
}

func sourceToTargetTransparentContainer(b []byte, sourceNGRANNode bool) SourceToTargetTransparentContainer {
	out := SourceToTargetTransparentContainer{Hex: hex.EncodeToString(b)}

	if !sourceNGRANNode {
		return out
	}

	c, err := ngap.ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b)
	if err != nil {
		out.Error = err.Error()

		return out
	}

	out.SourceNGRANNodeToTargetNGRANNode = sourceNGRANNodeToTargetNGRANNode(c)

	return out
}

func sourceNGRANNodeToTargetNGRANNode(c *ngap.SourceNGRANNodeToTargetNGRANNodeTransparentContainer) *SourceNGRANNodeToTargetNGRANNodeTransparentContainer {
	out := &SourceNGRANNodeToTargetNGRANNodeTransparentContainer{
		RRCContainer:         hex.EncodeToString(c.RRCContainer),
		TargetCellID:         ngranCGI(c.TargetCellID),
		UEHistoryInformation: make([]LastVisitedCell, 0, len(c.UEHistoryInformation)),
	}

	for _, it := range c.PDUSessionResourceInformationList {
		s := PDUSessionResourceInformation{
			PDUSessionID:           int64(it.PDUSessionID),
			QosFlowInformationList: make([]QosFlowInformation, 0, len(it.QosFlowInformationList)),
		}

		for _, f := range it.QosFlowInformationList {
			s.QosFlowInformationList = append(s.QosFlowInformationList, QosFlowInformation{
				QosFlowIdentifier: int64(f.QosFlowIdentifier),
				DLForwarding:      dlForwarding(f.DLForwarding),
			})
		}

		for _, d := range it.DRBsToQosFlowsMappingList {
			m := DRBToQosFlowsMapping{DRBID: int64(d.DRBID), AssociatedQosFlowList: make([]MappedAssociatedQosFlow, 0, len(d.AssociatedQosFlowList))}

			for _, f := range d.AssociatedQosFlowList {
				a := MappedAssociatedQosFlow{QosFlowIdentifier: int64(f.QosFlowIdentifier)}

				if f.QosFlowMappingIndication != nil {
					e := utils.NamedEnum(uint8(*f.QosFlowMappingIndication), f.QosFlowMappingIndication.Name())
					a.QosFlowMappingIndication = &e
				}

				m.AssociatedQosFlowList = append(m.AssociatedQosFlowList, a)
			}

			s.DRBsToQosFlowsMappingList = append(s.DRBsToQosFlowsMappingList, m)
		}

		out.PDUSessionResourceInformationList = append(out.PDUSessionResourceInformationList, s)
	}

	for _, it := range c.ERABInformationList {
		out.ERABInformationList = append(out.ERABInformationList, ERABInformation{
			ERABID:       int64(it.ERABID),
			DLForwarding: dlForwarding(it.DLForwarding),
		})
	}

	if c.IndexToRFSP != nil {
		v := uint16(*c.IndexToRFSP)
		out.IndexToRFSP = &v
	}

	for _, it := range c.UEHistoryInformation {
		out.UEHistoryInformation = append(out.UEHistoryInformation, lastVisitedCell(it.LastVisitedCellInformation))
	}

	return out
}

func dlForwarding(d *ngap.DLForwarding) *utils.EnumField {
	if d == nil {
		return nil
	}

	e := utils.NamedEnum(uint8(*d), d.Name())

	return &e
}

func ngranCGI(c ngap.NGRANCGI) NGRANCGI {
	var out NGRANCGI

	switch {
	case c.NRCGI != nil:
		v := nrCGI(*c.NRCGI)
		out.NRCGI = &v
	case c.EUTRACGI != nil:
		v := eutraCGI(*c.EUTRACGI)
		out.EUTRACGI = &v
	default:
		out.Error = "unsupported NGRAN-CGI choice-Extensions alternative"
	}

	return out
}

func lastVisitedCell(c ngap.LastVisitedCellInformation) LastVisitedCell {
	var out LastVisitedCell

	switch {
	case c.NGRANCell != nil:
		n := &LastVisitedNGRANCell{
			GlobalCellID:       ngranCGI(c.NGRANCell.GlobalCellID),
			CellSize:           utils.NamedEnum(uint8(c.NGRANCell.CellType.CellSize), c.NGRANCell.CellType.CellSize.Name()),
			TimeUEStayedInCell: uint16(c.NGRANCell.TimeUEStayedInCell),
		}

		if g := c.NGRANCell.TimeUEStayedInCellEnhancedGranularity; g != nil {
			v := uint16(*g)
			n.TimeUEStayedInCellEnhancedGranularity = &v
		}

		if h := c.NGRANCell.HOCauseValue; h != nil {
			v := cause(*h)
			n.HOCauseValue = &v
		}

		out.NGRANCell = n
	case c.EUTRANCell != nil:
		v := hex.EncodeToString(*c.EUTRANCell)
		out.EUTRANCell = &v
	case c.UTRANCell != nil:
		v := hex.EncodeToString(*c.UTRANCell)
		out.UTRANCell = &v
	case c.GERANCell != nil:
		v := hex.EncodeToString(*c.GERANCell)
		out.GERANCell = &v
	}

	return out
}
