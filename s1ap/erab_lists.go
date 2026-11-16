// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type ERABToBeSwitchedULItem struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

type ERABToBeSetupListCtxtSUReq []ERABToBeSetupItemCtxtSUReq

func (l ERABToBeSetupListCtxtSUReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSetupItemCtxtSUReq, CriticalityReject, l)
}

func (l *ERABToBeSetupListCtxtSUReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeSetupItemCtxtSUReq](r, enc, maxnoofERABs, IDERABToBeSetupItemCtxtSUReq, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABSetupListCtxtSURes []ERABSetupItemCtxtSURes

func (l ERABSetupListCtxtSURes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABSetupItemCtxtSURes, CriticalityIgnore, l)
}

func (l *ERABSetupListCtxtSURes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABSetupItemCtxtSURes](r, enc, maxnoofERABs, IDERABSetupItemCtxtSURes, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABToBeSetupListBearerSUReq []ERABToBeSetupItemBearerSUReq

func (l ERABToBeSetupListBearerSUReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSetupItemBearerSUReq, CriticalityReject, l)
}

func (l *ERABToBeSetupListBearerSUReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeSetupItemBearerSUReq](r, enc, maxnoofERABs, IDERABToBeSetupItemBearerSUReq, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABSetupListBearerSURes []ERABSetupItemBearerSURes

func (l ERABSetupListBearerSURes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABSetupItemBearerSURes, CriticalityIgnore, l)
}

func (l *ERABSetupListBearerSURes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABSetupItemBearerSURes](r, enc, maxnoofERABs, IDERABSetupItemBearerSURes, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABToBeModifiedListBearerModReq []ERABToBeModifiedItemBearerModReq

func (l ERABToBeModifiedListBearerModReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModReq, CriticalityReject, l)
}

func (l *ERABToBeModifiedListBearerModReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeModifiedItemBearerModReq](r, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModReq, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABToBeModifiedListBearerModInd []ERABToBeModifiedItemBearerModInd

func (l ERABToBeModifiedListBearerModInd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModInd, CriticalityReject, l)
}

func (l *ERABToBeModifiedListBearerModInd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeModifiedItemBearerModInd](r, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModInd, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABNotToBeModifiedListBearerModInd []ERABNotToBeModifiedItemBearerModInd

func (l ERABNotToBeModifiedListBearerModInd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABNotToBeModifiedItemBearerModInd, CriticalityReject, l)
}

func (l *ERABNotToBeModifiedListBearerModInd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABNotToBeModifiedItemBearerModInd](r, enc, maxnoofERABs, IDERABNotToBeModifiedItemBearerModInd, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABToBeSetupListHOReq []ERABToBeSetupItemHOReq

func (l ERABToBeSetupListHOReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSetupItemHOReq, CriticalityReject, l)
}

func (l *ERABToBeSetupListHOReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeSetupItemHOReq](r, enc, maxnoofERABs, IDERABToBeSetupItemHOReq, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABAdmittedList []ERABAdmittedItem

func (l ERABAdmittedList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABAdmittedItem, CriticalityIgnore, l)
}

func (l *ERABAdmittedList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABAdmittedItem](r, enc, maxnoofERABs, IDERABAdmittedItem, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABSubjecttoDataForwardingList []ERABDataForwardingItem

func (l ERABSubjecttoDataForwardingList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABDataForwardingItem, CriticalityIgnore, l)
}

func (l *ERABSubjecttoDataForwardingList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABDataForwardingItem](r, enc, maxnoofERABs, IDERABDataForwardingItem, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABToBeSwitchedDLList []ERABToBeSwitchedDLItem

func (l ERABToBeSwitchedDLList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSwitchedDLItem, CriticalityReject, l)
}

func (l *ERABToBeSwitchedDLList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeSwitchedDLItem](r, enc, maxnoofERABs, IDERABToBeSwitchedDLItem, CriticalityReject)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ERABToBeSwitchedULList []ERABToBeSwitchedULItem

func (l ERABToBeSwitchedULList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSwitchedULItem, CriticalityIgnore, l)
}

func (l *ERABToBeSwitchedULList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeSwitchedULItem](r, enc, maxnoofERABs, IDERABToBeSwitchedULItem, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l *ERABToBeSetupListCtxtSUReq) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeSetupItemCtxtSUReq, CriticalityReject, l)
}

func (l *ERABSetupListCtxtSURes) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABSetupItemCtxtSURes, CriticalityIgnore, l)
}

func (l *ERABToBeSetupListBearerSUReq) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeSetupItemBearerSUReq, CriticalityReject, l)
}

func (l *ERABSetupListBearerSURes) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABSetupItemBearerSURes, CriticalityIgnore, l)
}

func (l *ERABToBeModifiedListBearerModReq) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeModifiedItemBearerModReq, CriticalityReject, l)
}

func (l *ERABToBeModifiedListBearerModInd) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeModifiedItemBearerModInd, CriticalityReject, l)
}

func (l *ERABNotToBeModifiedListBearerModInd) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABNotToBeModifiedItemBearerModInd, CriticalityReject, l)
}

func (l *ERABToBeSetupListHOReq) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeSetupItemHOReq, CriticalityReject, l)
}

func (l *ERABAdmittedList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABAdmittedItem, CriticalityIgnore, l)
}

func (l *ERABSubjecttoDataForwardingList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABDataForwardingItem, CriticalityIgnore, l)
}

func (l *ERABToBeSwitchedDLList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeSwitchedDLItem, CriticalityReject, l)
}

func (l *ERABToBeSwitchedULList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeSwitchedULItem, CriticalityIgnore, l)
}
