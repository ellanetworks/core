// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
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
	RRCContainer                                  RRCContainer
	PDUSessionResourceInformationList             PDUSessionResourceInformationList `per:",optional"`
	ERABInformationList                           ERABInformationList               `per:",optional"`
	TargetCellID                                  NGRANCGI
	IndexToRFSP                                   *IndexToRFSP `per:",optional"`
	UEHistoryInformation                          UEHistoryInformation
	SgNBUEX2APID                                  *SgNBUEX2APID                                 `per:",extie:IDSgNBUEX2APID,crit:ignore"`
	UEHistoryInformationFromTheUE                 *UEHistoryInformationFromTheUE                `per:",extie:IDUEHistoryInformationFromTheUE,crit:ignore"`
	SourceNodeID                                  *SourceNodeID                                 `per:",extie:IDSourceNodeID,crit:ignore"`
	UEContextReferenceAtSource                    *RANUENGAPID                                  `per:",extie:IDUEContextReferenceAtSource,crit:ignore"`
	MBSActiveSessionInformationSourcetoTargetList MBSActiveSessionInformationSourcetoTargetList `per:",extie:IDMBSActiveSessionInformationSourcetoTargetList,crit:ignore"`
	QMCConfigInfo                                 *QMCConfigInfo                                `per:",extie:IDQMCConfigInfo,crit:ignore"`
	NGAPIESupportInformationRequestList           NGAPIESupportInformationRequestList           `per:",extie:IDNGAPIESupportInformationRequestList,crit:ignore"`
	CandidateRelayUEInformationList               CandidateRelayUEInformationList               `per:",extie:IDCandidateRelayUEInformationList,crit:reject"`
	TimeBasedHandoverInformation                  *TimeBasedHandoverInformation                 `per:",extie:IDTimeBasedHandoverInformation,crit:ignore"`
	SourceSNtoTargetSNQMCInfo                     *QMCConfigInfo                                `per:",extie:IDSourceSNtoTargetSNQMCInfo,crit:ignore"`
	iEExtensions                                  extensionMeta                                 `per:",exttable"`
	extensions                                    per.ExtensionAdditions                        `per:",extraw"`
}

func (c *SourceNGRANNodeToTargetNGRANNodeTransparentContainer) Marshal() (SourceToTargetTransparentContainer, error) {
	w := per.NewWriter()

	if err := c.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return SourceToTargetTransparentContainer(w.Bytes()), nil
}

func DecodeSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b []byte) (*SourceNGRANNodeToTargetNGRANNodeTransparentContainer, error) {
	c, err := decodeTransfer[SourceNGRANNodeToTargetNGRANNodeTransparentContainer](b)
	if err != nil {
		return nil, fmt.Errorf("ngap: SourceNGRANNode-ToTargetNGRANNode-TransparentContainer: %w", err)
	}

	return c, nil
}

func ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b []byte) (*SourceNGRANNodeToTargetNGRANNodeTransparentContainer, error) {
	c, err := parseTransfer[SourceNGRANNodeToTargetNGRANNodeTransparentContainer](b)
	if err != nil {
		return nil, fmt.Errorf("ngap: SourceNGRANNode-ToTargetNGRANNode-TransparentContainer: %w", err)
	}

	return c, nil
}

func (c *SourceNGRANNodeToTargetNGRANNodeTransparentContainer) Diagnostics() Diagnostics {
	return diagnosticsOf(c)
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
	iEExtensions              extensionMeta             `per:",exttable"`
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
	QosFlowIdentifier     QosFlowIdentifier
	DLForwarding          *DLForwarding          `per:",optional"`
	ULForwarding          *ULForwarding          `per:",extie:IDULForwarding,crit:ignore"`
	SourceTNLAddrInfo     *TransportLayerAddress `per:",extie:IDSourceTNLAddrInfo,crit:ignore"`
	SourceNodeTNLAddrInfo *TransportLayerAddress `per:",extie:IDSourceNodeTNLAddrInfo,crit:ignore"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
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
	DAPSRequestInfo       *DAPSRequestInfo       `per:",extie:IDDAPSRequestInfo,crit:ignore"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
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
)

type ERABInformationItem struct {
	ERABID                ERABID
	DLForwarding          *DLForwarding          `per:",optional"`
	SourceTNLAddrInfo     *TransportLayerAddress `per:",extie:IDSourceTNLAddrInfo,crit:ignore"`
	SourceNodeTNLAddrInfo *TransportLayerAddress `per:",extie:IDSourceNodeTNLAddrInfo,crit:ignore"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
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
	NRCGI           *NRCGI           `per:",choice:0"`
	EUTRACGI        *EUTRACGI        `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type IndexToRFSP uint16

func (i IndexToRFSP) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 256, int64(i))
}

func (i *IndexToRFSP) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[IndexToRFSP](r, enc, 1, 256)
	if err != nil {
		return err
	}

	*i = v

	return nil
}

func (v IndexToRFSP) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 1, 256) }

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
	iEExtensions               extensionMeta          `per:",exttable"`
	extensions                 per.ExtensionAdditions `per:",extraw"`
}

type LastVisitedCellInformation struct {
	NGRANCell       *LastVisitedNGRANCellInformation  `per:",choice:0"`
	EUTRANCell      *LastVisitedEUTRANCellInformation `per:",choice:1"`
	UTRANCell       *LastVisitedUTRANCellInformation  `per:",choice:2"`
	GERANCell       *LastVisitedGERANCellInformation  `per:",choice:3"`
	ChoiceExtension *SingleContainer                  `per:",choice:4,extie"`
}

type LastVisitedNGRANCellInformation struct {
	GlobalCellID                          NGRANCGI
	CellType                              CellType
	TimeUEStayedInCell                    TimeUEStayedInCell
	TimeUEStayedInCellEnhancedGranularity *TimeUEStayedInCellEnhancedGranularity `per:",optional"`
	HOCauseValue                          *Cause                                 `per:",optional"`
	LastVisitedPSCellList                 LastVisitedPSCellList                  `per:",extie:IDLastVisitedPSCellList,crit:ignore"`
	iEExtensions                          extensionMeta                          `per:",exttable"`
	extensions                            per.ExtensionAdditions                 `per:",extraw"`
}

type CellType struct {
	CellSize     CellSize
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CellSize uint8

const (
	CellSizeVerySmall CellSize = iota
	CellSizeSmall
	CellSizeMedium
	CellSizeLarge
)

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
