// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

const maxnoofCellsinUEHistoryInfo = 16

const (
	lastVisitedCellItemEUTRAN = iota
	lastVisitedCellItemUTRAN
	lastVisitedCellItemGERAN

	lastVisitedCellItemRootCount = 3
)

const lastVisitedCellItemNGRANAddition = 0

const lastVisitedGERANCellInformationRootCount = 1

type SourceeNBToTargeteNBTransparentContainer struct {
	RRCContainer              RRCContainer
	ERABInformationList       ERABInformationList `per:",optional"`
	TargetCellID              EUTRANCGI
	SubscriberProfileIDforRFP *SubscriberProfileIDforRFP `per:",optional"`
	UEHistoryInformation      UEHistoryInformation
	iEExtensions              *transparentIEExtensions `per:",optional"`
	extensions                per.ExtensionAdditions   `per:",extraw"`
}

func (c *SourceeNBToTargeteNBTransparentContainer) Marshal() (TransparentContainer, error) {
	w := per.NewWriter()

	if err := c.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransparentContainer(w.Bytes()), nil
}

func ParseSourceeNBToTargeteNBTransparentContainer(b []byte) (*SourceeNBToTargeteNBTransparentContainer, error) {
	var c SourceeNBToTargeteNBTransparentContainer

	if err := c.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, fmt.Errorf("s1ap: SourceeNB-ToTargeteNB-TransparentContainer: %w", err)
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

type DLForwarding uint8

const (
	DLForwardingProposed DLForwarding = iota

	dlForwardingRootCount = 1
)

func (d DLForwarding) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, dlForwardingRootCount, int64(d), "DL-Forwarding")
}

func (d *DLForwarding) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, dlForwardingRootCount, "DL-Forwarding")
	if err != nil {
		return err
	}

	*d = DLForwarding(idx)

	return nil
}

type ERABInformationListItem struct {
	ERABID       ERABID
	DLForwarding *DLForwarding            `per:",optional"`
	iEExtensions *transparentIEExtensions `per:",optional"`
	extensions   per.ExtensionAdditions   `per:",extraw"`
}

type ERABInformationList []ERABInformationListItem

func (l ERABInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABInformationListItem, CriticalityIgnore, l)
}

func (l *ERABInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABInformationListItem](r, enc, maxnoofERABs, IDERABInformationListItem, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SubscriberProfileIDforRFP uint16

func (s SubscriberProfileIDforRFP) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true}, int64(s))
}

func (s *SubscriberProfileIDforRFP) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true})
	if err != nil {
		return err
	}

	*s = SubscriberProfileIDforRFP(v)

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
	EUTRANCell *LastVisitedEUTRANCellInformation
	UTRANCell  *LastVisitedUTRANCellInformation
	GERANCell  *LastVisitedGERANCellInformation
	NGRANCell  *LastVisitedNGRANCellInformation
	extension  *choiceExtensionAlternative
}

func (l *LastVisitedCellItem) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := choiceAlternative("LastVisitedCell-Item", l.EUTRANCell != nil, l.UTRANCell != nil, l.GERANCell != nil, l.NGRANCell != nil, l.extension != nil)
	if err != nil {
		return err
	}

	switch idx {
	case lastVisitedCellItemEUTRAN:
		if err := encodeExtensibleChoice(w, enc, lastVisitedCellItemRootCount, idx); err != nil {
			return err
		}

		return l.EUTRANCell.MarshalPER(w, enc)
	case lastVisitedCellItemUTRAN:
		if err := encodeExtensibleChoice(w, enc, lastVisitedCellItemRootCount, idx); err != nil {
			return err
		}

		return l.UTRANCell.MarshalPER(w, enc)
	case lastVisitedCellItemGERAN:
		if err := encodeExtensibleChoice(w, enc, lastVisitedCellItemRootCount, idx); err != nil {
			return err
		}

		return l.GERANCell.MarshalPER(w, enc)
	case lastVisitedCellItemRootCount:
		return encodeChoiceAddition(w, enc, lastVisitedCellItemNGRANAddition, l.NGRANCell)
	default:
		return l.extension.MarshalPER(w, enc)
	}
}

func (l *LastVisitedCellItem) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	*l = LastVisitedCellItem{}

	if isExt {
		alt, err := decodeChoiceAddition(r, enc)
		if err != nil {
			return err
		}

		if alt.index != lastVisitedCellItemNGRANAddition {
			l.extension = alt

			return nil
		}

		var v LastVisitedNGRANCellInformation

		inner := per.NewReader(alt.value)
		if err := v.UnmarshalPER(inner, enc); err != nil {
			return err
		}

		if inner.Bits() >= 8 {
			return fmt.Errorf("s1ap: LastVisitedCell-Item nG-RAN-Cell open type carries %d trailing bits", inner.Bits())
		}

		l.NGRANCell = &v

		return nil
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, lastVisitedCellItemRootCount-1)
	if err != nil {
		return err
	}

	switch idx {
	case lastVisitedCellItemEUTRAN:
		var v LastVisitedEUTRANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.EUTRANCell = &v
	case lastVisitedCellItemUTRAN:
		var v LastVisitedUTRANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.UTRANCell = &v
	default:
		var v LastVisitedGERANCellInformation
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		l.GERANCell = &v
	}

	return nil
}

type LastVisitedEUTRANCellInformation struct {
	GlobalCellID       EUTRANCGI
	CellType           CellType
	TimeUEStayedInCell TimeUEStayedInCell
	iEExtensions       *transparentIEExtensions `per:",optional"`
	extensions         per.ExtensionAdditions   `per:",extraw"`
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
	return encodeRootEnumerated(w, enc, cellSizeRootCount, int64(c), "Cell-Size")
}

func (c *CellSize) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, cellSizeRootCount, "Cell-Size")
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

type LastVisitedNGRANCellInformation []byte

func (c LastVisitedNGRANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *LastVisitedNGRANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = LastVisitedNGRANCellInformation(b)

	return nil
}

type LastVisitedGERANCellInformation struct {
	extension *choiceExtensionAlternative
}

func (g *LastVisitedGERANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if g.extension != nil {
		return g.extension.MarshalPER(w, enc)
	}

	return encodeExtensibleChoice(w, enc, lastVisitedGERANCellInformationRootCount, 0)
}

func (g *LastVisitedGERANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	*g = LastVisitedGERANCellInformation{}

	if !isExt {
		_, err := per.DecodeConstrainedWholeNumber(r, enc, 0, lastVisitedGERANCellInformationRootCount-1)

		return err
	}

	g.extension, err = decodeChoiceAddition(r, enc)

	return err
}

type choiceExtensionAlternative struct {
	index int64
	value []byte
}

func (a *choiceExtensionAlternative) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(true)

	if err := per.EncodeNormallySmall(w, enc, a.index); err != nil {
		return err
	}

	return per.EncodeOpenTypeBytes(w, enc, a.value)
}

func encodeChoiceAddition(w *per.Writer, enc per.Encoding, index int64, m per.Marshaler) error {
	w.WriteBit(true)

	if err := per.EncodeNormallySmall(w, enc, index); err != nil {
		return err
	}

	return per.EncodeOpenType(w, enc, m)
}

func decodeChoiceAddition(r *per.Reader, enc per.Encoding) (*choiceExtensionAlternative, error) {
	index, err := per.DecodeNormallySmall(r, enc)
	if err != nil {
		return nil, err
	}

	value, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return nil, err
	}

	if len(value) == 0 {
		return nil, fmt.Errorf("s1ap: empty open type for CHOICE extension alternative %d", index)
	}

	return &choiceExtensionAlternative{index: index, value: value}, nil
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
