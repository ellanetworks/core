// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

func TestContainerDecodeErrorsAreTyped(t *testing.T) {
	_, err := DecodeSourceeNBToTargeteNBTransparentContainer([]byte{0x80})

	var tse *TransferSyntaxError
	if !errors.As(err, &tse) {
		t.Fatalf("truncated container: err = %v (%T), want *TransferSyntaxError", err, err)
	}

	in := &SourceeNBToTargeteNBTransparentContainer{
		RRCContainer:         RRCContainer{0x80},
		TargetCellID:         EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 1},
		ERABInformationList:  ERABInformationList{{ERABID: 1}},
		UEHistoryInformation: UEHistoryInformation{{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01})}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeSourceeNBToTargeteNBTransparentContainer(append(b, 0x00)); !errors.As(err, &tse) || !errors.Is(err, per.ErrTrailing) {
		t.Fatalf("trailing octet: err = %v, want *TransferSyntaxError wrapping per.ErrTrailing", err)
	}
}

func TestContainerWhoseRootListHoldsOnlyUnknownIEsIsAnAbstractSyntaxError(t *testing.T) {
	in := &SourceeNBToTargeteNBTransparentContainer{
		RRCContainer:         RRCContainer{0x80},
		TargetCellID:         EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 1},
		ERABInformationList:  ERABInformationList{{ERABID: 1}},
		UEHistoryInformation: UEHistoryInformation{{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01})}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	item, err := per.Marshal(&in.ERABInformationList[0], per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	foreign := bytesReplace(t, b, item)

	_, err = DecodeSourceeNBToTargeteNBTransparentContainer(foreign)

	var ase *ap.AbstractSyntaxError[ProtocolIEID]
	if !errors.As(err, &ase) || len(ase.IEs) != 1 || ase.IEs[0].ID != idForeignListItem {
		t.Fatalf("err = %v, want an *ap.AbstractSyntaxError naming the unknown IE", err)
	}
}

func bytesReplace(t *testing.T, b, item []byte) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := encodeContainerField(w, per.Aligned, ieField{ID: IDERABInformationListItem, Criticality: CriticalityIgnore, Value: apimpl.RawValue(item)}); err != nil {
		t.Fatal(err)
	}

	known := w.Bytes()

	w = per.NewWriter()
	if err := encodeContainerField(w, per.Aligned, ieField{ID: idForeignListItem, Criticality: CriticalityIgnore, Value: apimpl.RawValue(item)}); err != nil {
		t.Fatal(err)
	}

	out := make([]byte, 0, len(b))

	for i := 0; i < len(b); i++ {
		if i+len(known) <= len(b) && string(b[i:i+len(known)]) == string(known) {
			out = append(out, w.Bytes()...)
			out = append(out, b[i+len(known):]...)

			return out
		}

		out = append(out, b[i])
	}

	t.Fatal("list item not found in the encoding")

	return nil
}
