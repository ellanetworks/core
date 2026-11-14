// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

const (
	maxnoofERABs                = 256
	maxnoofCellsinUEHistoryInfo = 16
)

const (
	ngranCGINR = iota
	ngranCGIEUTRA
	ngranCGIChoiceExtensions

	ngranCGIAlternatives = 3
)

const (
	lastVisitedCellInformationNGRAN = iota
	lastVisitedCellInformationEUTRAN
	lastVisitedCellInformationUTRAN
	lastVisitedCellInformationGERAN
	lastVisitedCellInformationChoiceExtensions

	lastVisitedCellInformationAlternatives = 5
)

type SourceNGRANNodeToTargetNGRANNodeTransparentContainer struct {
	RRCContainer                      RRCContainer
	PDUSessionResourceInformationList PDUSessionResourceInformationList `per:",optional"`
	ERABInformationList               ERABInformationList               `per:",optional"`
	TargetCellID                      NGRANCGI
	IndexToRFSP                       *IndexToRFSP `per:",optional"`
	UEHistoryInformation              UEHistoryInformation
	iEExtensions                      *transparentIEExtensions `per:",optional"`
	extensions                        per.ExtensionAdditions   `per:",extraw"`
}

func (c *SourceNGRANNodeToTargetNGRANNodeTransparentContainer) Marshal() (SourceToTargetTransparentContainer, error) {
	w := per.NewWriter()

	if err := c.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return SourceToTargetTransparentContainer(w.Bytes()), nil
}

func ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b []byte) (*SourceNGRANNodeToTargetNGRANNodeTransparentContainer, error) {
	var c SourceNGRANNodeToTargetNGRANNodeTransparentContainer

	if err := c.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, fmt.Errorf("ngap: SourceNGRANNode-ToTargetNGRANNode-TransparentContainer: %w", err)
	}

	return &c, nil
}

type RRCContainer []byte

func (c RRCContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *RRCContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = RRCContainer(b)

	return nil
}

type PDUSessionResourceInformationItem struct {
	PDUSessionID              PDUSessionID
	QosFlowInformationList    QosFlowInformationList
	DRBsToQosFlowsMappingList DRBsToQosFlowsMappingList `per:",optional"`
	iEExtensions              *transparentIEExtensions  `per:",optional"`
	extensions                per.ExtensionAdditions    `per:",extraw"`
}

type PDUSessionResourceInformationList []PDUSessionResourceInformationItem

func (l PDUSessionResourceInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceInformationItem(l))
}

func (l *PDUSessionResourceInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceInformationItem](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QosFlowInformationItem struct {
	QosFlowIdentifier QosFlowIdentifier
	DLForwarding      *DLForwarding            `per:",optional"`
	iEExtensions      *transparentIEExtensions `per:",optional"`
	extensions        per.ExtensionAdditions   `per:",extraw"`
}

type QosFlowInformationList []QosFlowInformationItem

func (l QosFlowInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowInformationItem(l))
}

func (l *QosFlowInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowInformationItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type DRBsToQosFlowsMappingItem struct {
	DRBID                 DRBID
	AssociatedQosFlowList AssociatedQosFlowList
	iEExtensions          *transparentIEExtensions `per:",optional"`
	extensions            per.ExtensionAdditions   `per:",extraw"`
}

type DRBsToQosFlowsMappingList []DRBsToQosFlowsMappingItem

func (l DRBsToQosFlowsMappingList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofDRBs, []DRBsToQosFlowsMappingItem(l))
}

func (l *DRBsToQosFlowsMappingList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[DRBsToQosFlowsMappingItem](r, enc, 1, maxnoofDRBs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type DLForwarding uint8

const (
	DLForwardingProposed DLForwarding = iota

	dlForwardingRootCount = 1
)

func (d DLForwarding) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, dlForwardingRootCount, int64(d), "DLForwarding")
}

func (d *DLForwarding) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, dlForwardingRootCount, "DLForwarding")
	if err != nil {
		return err
	}

	*d = DLForwarding(idx)

	return nil
}

type ERABInformationItem struct {
	ERABID       ERABID
	DLForwarding *DLForwarding            `per:",optional"`
	iEExtensions *transparentIEExtensions `per:",optional"`
	extensions   per.ExtensionAdditions   `per:",extraw"`
}

type ERABInformationList []ERABInformationItem

func (l ERABInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofERABs, []ERABInformationItem(l))
}

func (l *ERABInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ERABInformationItem](r, enc, 1, maxnoofERABs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NGRANCGI struct {
	NRCGI           *NRCGI
	EUTRACGI        *EUTRACGI
	choiceExtension *rawIE
}

func (c *NGRANCGI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := chosenAlternative("NGRAN-CGI", c.NRCGI != nil, c.EUTRACGI != nil, c.choiceExtension != nil)
	if err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ngranCGIAlternatives-1, idx); err != nil {
		return err
	}

	switch idx {
	case ngranCGINR:
		return c.NRCGI.MarshalPER(w, enc)
	case ngranCGIEUTRA:
		return c.EUTRACGI.MarshalPER(w, enc)
	default:
		return encodeContainerField(w, enc, c.choiceExtension.field())
	}
}

func (c *NGRANCGI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, ngranCGIAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: NGRAN-CGI choice: %w", err)
	}

	*c = NGRANCGI{}

	switch idx {
	case ngranCGINR:
		var v NRCGI
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		c.NRCGI = &v
	case ngranCGIEUTRA:
		var v EUTRACGI
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		c.EUTRACGI = &v
	default:
		c.choiceExtension, err = decodeTransparentChoiceExtension(r, enc)

		return err
	}

	return nil
}

type IndexToRFSP uint16

func (i IndexToRFSP) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 256, int64(i))
}

func (i *IndexToRFSP) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 1, 256, "IndexToRFSP")
	if err != nil {
		return err
	}

	*i = IndexToRFSP(v)

	return nil
}

type UEHistoryInformation []LastVisitedCellItem

func (u UEHistoryInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsinUEHistoryInfo, []LastVisitedCellItem(u))
}

func (u *UEHistoryInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[LastVisitedCellItem](r, enc, 1, maxnoofCellsinUEHistoryInfo)
	if err != nil {
		return err
	}

	*u = items

	return nil
}

type LastVisitedCellItem struct {
	LastVisitedCellInformation LastVisitedCellInformation
	iEExtensions               *transparentIEExtensions `per:",optional"`
	extensions                 per.ExtensionAdditions   `per:",extraw"`
}

type LastVisitedCellInformation struct {
	NGRANCell       *LastVisitedNGRANCellInformation
	EUTRANCell      *LastVisitedEUTRANCellInformation
	UTRANCell       *LastVisitedUTRANCellInformation
	GERANCell       *LastVisitedGERANCellInformation
	choiceExtension *rawIE
}

func (l *LastVisitedCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := chosenAlternative("LastVisitedCellInformation", l.NGRANCell != nil, l.EUTRANCell != nil, l.UTRANCell != nil, l.GERANCell != nil, l.choiceExtension != nil)
	if err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, lastVisitedCellInformationAlternatives-1, idx); err != nil {
		return err
	}

	switch idx {
	case lastVisitedCellInformationNGRAN:
		return l.NGRANCell.MarshalPER(w, enc)
	case lastVisitedCellInformationEUTRAN:
		return l.EUTRANCell.MarshalPER(w, enc)
	case lastVisitedCellInformationUTRAN:
		return l.UTRANCell.MarshalPER(w, enc)
	case lastVisitedCellInformationGERAN:
		return l.GERANCell.MarshalPER(w, enc)
	default:
		return encodeContainerField(w, enc, l.choiceExtension.field())
	}
}

func (l *LastVisitedCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, lastVisitedCellInformationAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: LastVisitedCellInformation choice: %w", err)
	}

	*l = LastVisitedCellInformation{}

	switch idx {
	case lastVisitedCellInformationNGRAN:
		var v LastVisitedNGRANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.NGRANCell = &v
	case lastVisitedCellInformationEUTRAN:
		var v LastVisitedEUTRANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.EUTRANCell = &v
	case lastVisitedCellInformationUTRAN:
		var v LastVisitedUTRANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.UTRANCell = &v
	case lastVisitedCellInformationGERAN:
		var v LastVisitedGERANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.GERANCell = &v
	default:
		l.choiceExtension, err = decodeTransparentChoiceExtension(r, enc)

		return err
	}

	return nil
}

type LastVisitedNGRANCellInformation struct {
	GlobalCellID                          NGRANCGI
	CellType                              CellType
	TimeUEStayedInCell                    TimeUEStayedInCell
	TimeUEStayedInCellEnhancedGranularity *TimeUEStayedInCellEnhancedGranularity `per:",optional"`
	HOCauseValue                          *Cause                                 `per:",optional"`
	iEExtensions                          *transparentIEExtensions               `per:",optional"`
	extensions                            per.ExtensionAdditions                 `per:",extraw"`
}

type CellType struct {
	CellSize     CellSize
	iEExtensions *transparentIEExtensions `per:",optional"`
	extensions   per.ExtensionAdditions   `per:",extraw"`
}

type CellSize uint8

const (
	CellSizeVerySmall CellSize = iota
	CellSizeSmall
	CellSizeMedium
	CellSizeLarge

	cellSizeRootCount = 4
)

func (c CellSize) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, cellSizeRootCount, int64(c), "CellSize")
}

func (c *CellSize) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, cellSizeRootCount, "CellSize")
	if err != nil {
		return err
	}

	*c = CellSize(idx)

	return nil
}

type TimeUEStayedInCell uint16

func (t TimeUEStayedInCell) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true}, int64(t))
}

func (t *TimeUEStayedInCell) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true})
	if err != nil {
		return err
	}

	*t = TimeUEStayedInCell(v)

	return nil
}

type TimeUEStayedInCellEnhancedGranularity uint16

func (t TimeUEStayedInCellEnhancedGranularity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true}, int64(t))
}

func (t *TimeUEStayedInCellEnhancedGranularity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true})
	if err != nil {
		return err
	}

	*t = TimeUEStayedInCellEnhancedGranularity(v)

	return nil
}

type LastVisitedEUTRANCellInformation []byte

func (c LastVisitedEUTRANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *LastVisitedEUTRANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = LastVisitedEUTRANCellInformation(b)

	return nil
}

type LastVisitedUTRANCellInformation []byte

func (c LastVisitedUTRANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *LastVisitedUTRANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = LastVisitedUTRANCellInformation(b)

	return nil
}

type LastVisitedGERANCellInformation []byte

func (c LastVisitedGERANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *LastVisitedGERANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = LastVisitedGERANCellInformation(b)

	return nil
}

func decodeTransparentChoiceExtension(r *per.Reader, enc per.Encoding) (*rawIE, error) {
	id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return nil, err
	}

	crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
	if err != nil {
		return nil, err
	}

	value, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return nil, err
	}

	if len(value) == 0 {
		return nil, fmt.Errorf("ngap: empty open type for choice-Extensions IE %d", id)
	}

	return &rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: value}, nil
}

type transparentIEExtensions struct {
	data  []byte
	nbits int
}

func (e *transparentIEExtensions) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if enc == per.Aligned {
		w.AlignToByte()
	}

	w.WriteBitString(e.data, e.nbits)

	return nil
}

func (e *transparentIEExtensions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	if enc == per.Aligned {
		r.AlignToByte()
	}

	start := r.Offset()

	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, maxProtocolExtensions)
	if err != nil {
		return err
	}

	for range n {
		if _, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs); err != nil {
			return err
		}

		if _, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false); err != nil {
			return err
		}

		if err := per.SkipOpenType(r, enc); err != nil {
			return err
		}
	}

	e.data, e.nbits, err = r.Since(start)

	return err
}
