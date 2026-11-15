// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

//go:generate go run github.com/ellanetworks/core/per/cmd/pergen -profile profile -ieid ProtocolIEID

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Hand-written codecs for leaf types whose Go shape differs from their wire
// shape; pergen generates the SEQUENCEs built from them.

func (p PLMNIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 3, 3, true, true, false, p[:])
}

func (p *PLMNIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	copy(p[:], b)

	return nil
}

// tacMax is the largest value a three-octet TAC can hold.
const tacMax = 1<<24 - 1

func (t TAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if t > tacMax {
		return fmt.Errorf("ngap: TAC %d exceeds three octets", uint32(t))
	}

	return per.EncodeOctetString(w, enc, 3, 3, true, true, false,
		[]byte{byte(t >> 16), byte(t >> 8), byte(t)})
}

func (t *TAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	*t = TAC(uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]))

	return nil
}

func (t EPSTAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, []byte{byte(t >> 8), byte(t)})
}

func (t *EPSTAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	*t = EPSTAC(uint16(b[0])<<8 | uint16(b[1]))

	return nil
}

func (v GNBIDValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if v.Bits < gnbIDMinBits || v.Bits > gnbIDMaxBits {
		return fmt.Errorf("ngap: gNB-ID of %d bits outside SIZE(%d..%d)", v.Bits, gnbIDMinBits, gnbIDMaxBits)
	}

	if uint64(v.Value) >= 1<<uint(v.Bits) {
		return fmt.Errorf("ngap: gNB-ID %d exceeds its own %d bits", v.Value, v.Bits)
	}

	return per.EncodeBitString(w, enc, gnbIDMinBits, gnbIDMaxBits, true, true, false,
		uintToBits(uint64(v.Value), v.Bits), v.Bits)
}

func (v *GNBIDValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, gnbIDMinBits, gnbIDMaxBits, true, true, false)
	if err != nil {
		return err
	}

	*v = GNBIDValue{Value: uint32(bitsToUint(b, nbits)), Bits: nbits}

	return nil
}

func (v N3IWFIDValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), n3IWFIDBits)
}

func (v *N3IWFIDValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	u, err := decodeBitStringUint(r, enc, n3IWFIDBits)
	*v = N3IWFIDValue(u)

	return err
}

func (v MacroNgENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), macroNgENBIDBits)
}

func (v *MacroNgENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	u, err := decodeBitStringUint(r, enc, macroNgENBIDBits)
	*v = MacroNgENBID(u)

	return err
}

func (v ShortMacroNgENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), shortMacroNgENBIDBits)
}

func (v *ShortMacroNgENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	u, err := decodeBitStringUint(r, enc, shortMacroNgENBIDBits)
	*v = ShortMacroNgENBID(u)

	return err
}

func (v LongMacroNgENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), longMacroNgENBIDBits)
}

func (v *LongMacroNgENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	u, err := decodeBitStringUint(r, enc, longMacroNgENBIDBits)
	*v = LongMacroNgENBID(u)

	return err
}

func (v TNGFIDValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, tngfIDBits, tngfIDBits)
}

func (v *TNGFIDValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, tngfIDBits, tngfIDBits)
	*v = TNGFIDValue{b}

	return err
}

func (v TWIFIDValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, twifIDBits, twifIDBits)
}

func (v *TWIFIDValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, twifIDBits, twifIDBits)
	*v = TWIFIDValue{b}

	return err
}

func (v WAGFIDValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, wagfIDBits, wagfIDBits)
}

func (v *WAGFIDValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, wagfIDBits, wagfIDBits)
	*v = WAGFIDValue{b}

	return err
}

func (i RATRestrictionInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if i.extended {
		return i.raw.encode(w, enc, ratRestrictionInformationBits, ratRestrictionInformationBits)
	}

	return BitStringFromUint(uint64(i.Flags), ratRestrictionInformationBits).encode(w, enc,
		ratRestrictionInformationBits, ratRestrictionInformationBits)
}

func (i *RATRestrictionInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, ratRestrictionInformationBits, ratRestrictionInformationBits)
	if err != nil {
		return err
	}

	*i = RATRestrictionInformation{Flags: RATRestrictionFlags(append(b.Bytes(), 0)[0])}
	if b.Bits() != ratRestrictionInformationBits {
		i.extended, i.raw = true, b
	}

	return nil
}

func (e EquivalentPLMNs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNs, []PLMNIdentity(e))
}

func (e *EquivalentPLMNs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNIdentity](r, enc, 1, maxnoofEPLMNs)
	if err != nil {
		return err
	}

	*e = items

	return nil
}

func (f ForbiddenTACs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofForbTACs, []TAC(f))
}

func (f *ForbiddenTACs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofForbTACs)
	if err != nil {
		return err
	}

	*f = items

	return nil
}

func (f ForbiddenAreaInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []ForbiddenAreaInformationItem(f))
}

func (f *ForbiddenAreaInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ForbiddenAreaInformationItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	if err != nil {
		return err
	}

	*f = items

	return nil
}

func (l RATRestrictions) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []RATRestrictionsItem(l))
}

func (l *RATRestrictions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[RATRestrictionsItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (a AllowedTACs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAllowedAreas, []TAC(a))
}

func (a *AllowedTACs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofAllowedAreas)
	if err != nil {
		return err
	}

	*a = items

	return nil
}

func (n NotAllowedTACs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAllowedAreas, []TAC(n))
}

func (n *NotAllowedTACs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofAllowedAreas)
	if err != nil {
		return err
	}

	*n = items

	return nil
}

func (s ServiceAreaInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []ServiceAreaInformationItem(s))
}

func (s *ServiceAreaInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ServiceAreaInformationItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (t FiveGTMSI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	var b [4]byte

	binary.BigEndian.PutUint32(b[:], uint32(t))

	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, b[:])
}

func (t *FiveGTMSI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*t = FiveGTMSI(binary.BigEndian.Uint32(b))

	return nil
}

func (s SST) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 1, true, true, false, []byte{byte(s)})
}

func (s *SST) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 1, true, true, false)
	if err != nil {
		return err
	}

	*s = SST(b[0])

	return nil
}

func (s SD) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 3, 3, true, true, false, s[:])
}

func (s *SD) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	copy(s[:], b)

	return nil
}

// The GUAMI fields are BIT STRINGs of non-octet widths, so each is packed from
// an integer rather than copied from octets.

func (a AMFRegionID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), amfRegionIDBits)
}

func (a *AMFRegionID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, amfRegionIDBits)
	if err != nil {
		return err
	}

	*a = AMFRegionID(v)

	return nil
}

func (a AMFSetID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), amfSetIDBits)
}

func (a *AMFSetID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, amfSetIDBits)
	if err != nil {
		return err
	}

	*a = AMFSetID(v)

	return nil
}

func (a AMFPointer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), amfPointerBits)
}

func (a *AMFPointer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, amfPointerBits)
	if err != nil {
		return err
	}

	*a = AMFPointer(v)

	return nil
}

// A value wider than the field is an error rather than a silent truncation.
func encodeBitStringUint(w *per.Writer, enc per.Encoding, v uint64, nbits int) error {
	if nbits < 64 && v >= 1<<uint(nbits) {
		return fmt.Errorf("ngap: value %d exceeds %d-bit field", v, nbits)
	}

	return per.EncodeBitString(w, enc, int64(nbits), int64(nbits), true, true, false, uintToBits(v, nbits), nbits)
}

func decodeBitStringUint(r *per.Reader, enc per.Encoding, nbits int) (uint64, error) {
	b, _, err := per.DecodeBitString(r, enc, int64(nbits), int64(nbits), true, true, false)
	if err != nil {
		return 0, err
	}

	return bitsToUint(b, nbits), nil
}

func (a AllowedNSSAI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAllowedSNSSAIs, []AllowedNSSAIItem(a))
}

func (a *AllowedNSSAI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AllowedNSSAIItem](r, enc, 1, maxnoofAllowedSNSSAIs)
	if err != nil {
		return err
	}

	*a = items

	return nil
}

func (l PDUSessionResourceNotifyList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceNotifyItem(l))
}

func (l *PDUSessionResourceNotifyList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceNotifyItem](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceReleasedListNot) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceReleasedItemNot(l))
}

func (l *PDUSessionResourceReleasedListNot) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceReleasedItemNot](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceModifyListModInd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceModifyItemModInd(l))
}

func (l *PDUSessionResourceModifyListModInd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceModifyItemModInd](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceModifyListModCfm) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceModifyItemModCfm(l))
}

func (l *PDUSessionResourceModifyListModCfm) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceModifyItemModCfm](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToModifyListModCfm) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToModifyItemModCfm(l))
}

func (l *PDUSessionResourceFailedToModifyListModCfm) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToModifyItemModCfm](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceModifyListModReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceModifyItemModReq(l))
}

func (l *PDUSessionResourceModifyListModReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceModifyItemModReq](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceModifyListModRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceModifyItemModRes(l))
}

func (l *PDUSessionResourceModifyListModRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceModifyItemModRes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToModifyListModRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToModifyItemModRes(l))
}

func (l *PDUSessionResourceFailedToModifyListModRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToModifyItemModRes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceToReleaseListRelCmd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceToReleaseItemRelCmd(l))
}

func (l *PDUSessionResourceToReleaseListRelCmd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceToReleaseItemRelCmd](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceReleasedListRelRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceReleasedItemRelRes(l))
}

func (l *PDUSessionResourceReleasedListRelRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceReleasedItemRelRes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceSetupListSUReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceSetupItemSUReq(l))
}

func (l *PDUSessionResourceSetupListSUReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceSetupItemSUReq](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceSetupListSURes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceSetupItemSURes(l))
}

func (l *PDUSessionResourceSetupListSURes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceSetupItemSURes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToSetupListSURes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToSetupItemSURes(l))
}

func (l *PDUSessionResourceFailedToSetupListSURes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToSetupItemSURes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceSetupListCxtReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceSetupItemCxtReq(l))
}

func (l *PDUSessionResourceSetupListCxtReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceSetupItemCxtReq](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceSetupListCxtRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceSetupItemCxtRes(l))
}

func (l *PDUSessionResourceSetupListCxtRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceSetupItemCxtRes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToSetupListCxtRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToSetupItemCxtRes(l))
}

func (l *PDUSessionResourceFailedToSetupListCxtRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToSetupItemCxtRes](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToSetupListCxtFail) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToSetupItemCxtFail(l))
}

func (l *PDUSessionResourceFailedToSetupListCxtFail) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToSetupItemCxtFail](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceListCxtRelReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceItemCxtRelReq(l))
}

func (l *PDUSessionResourceListCxtRelReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceItemCxtRelReq](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceListCxtRelCpl) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceItemCxtRelCpl(l))
}

func (l *PDUSessionResourceListCxtRelCpl) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceItemCxtRelCpl](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (s SliceSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofSliceItems, []SliceSupportItem(s))
}

func (s *SliceSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SliceSupportItem](r, enc, 1, maxnoofSliceItems)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (b BroadcastPLMNList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofBPLMNs, []BroadcastPLMNItem(b))
}

func (b *BroadcastPLMNList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[BroadcastPLMNItem](r, enc, 1, maxnoofBPLMNs)
	if err != nil {
		return err
	}

	*b = items

	return nil
}

func (s SupportedTAList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTACs, []SupportedTAItem(s))
}

func (s *SupportedTAList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SupportedTAItem](r, enc, 1, maxnoofTACs)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (l NGRANTNLAssociationToRemoveList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTNLAssociations, []NGRANTNLAssociationToRemoveItem(l))
}

func (l *NGRANTNLAssociationToRemoveList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGRANTNLAssociationToRemoveItem](r, enc, 1, maxnoofTNLAssociations)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l UnavailableGUAMIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofServedGUAMIs, []UnavailableGUAMIItem(l))
}

func (l *UnavailableGUAMIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UnavailableGUAMIItem](r, enc, 1, maxnoofServedGUAMIs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (s ServedGUAMIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofServedGUAMIs, []ServedGUAMIItem(s))
}

func (s *ServedGUAMIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ServedGUAMIItem](r, enc, 1, maxnoofServedGUAMIs)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (l UEAssociatedLogicalNGConnectionList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofNGConnectionsToReset, []UEAssociatedLogicalNGConnectionItem(l))
}

func (l *UEAssociatedLogicalNGConnectionList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UEAssociatedLogicalNGConnectionItem](r, enc, 1, maxnoofNGConnectionsToReset)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (p PLMNSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPLMNs, []PLMNSupportItem(p))
}

func (p *PLMNSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNSupportItem](r, enc, 1, maxnoofPLMNs)
	if err != nil {
		return err
	}

	*p = items

	return nil
}

func marshalSeqOf[T any](w *per.Writer, enc per.Encoding, lb, ub int64, items []T) error {
	off := 0

	return per.EncodeLength(w, enc, lb, ub, true, int64(len(items)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			m, ok := any(&items[i]).(per.Marshaler)
			if !ok {
				return fmt.Errorf("ngap: %T does not implement per.Marshaler", items[i])
			}

			if err := m.MarshalPER(w, enc); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

func unmarshalSeqOf[T any](r *per.Reader, enc per.Encoding, lb, ub int64) ([]T, error) {
	var items []T

	err := per.DecodeLength(r, enc, lb, ub, true, func(count int64) error {
		start := len(items)
		items = append(items, make([]T, count)...)

		for i := int64(0); i < count; i++ {
			u, ok := any(&items[start+int(i)]).(per.Unmarshaler)
			if !ok {
				return fmt.Errorf("ngap: %T does not implement per.Unmarshaler", items[start+int(i)])
			}

			if err := u.UnmarshalPER(r, enc); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (l QosFlowSetupRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowSetupRequestItem(l))
}

func (l *QosFlowSetupRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowSetupRequestItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowAddOrModifyResponseList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowAddOrModifyResponseItem(l))
}

func (l *QosFlowAddOrModifyResponseList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowAddOrModifyResponseItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowListWithCause) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowWithCauseItem(l))
}

func (l *QosFlowListWithCause) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowWithCauseItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l AssociatedQosFlowList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []AssociatedQosFlowItem(l))
}

func (l *AssociatedQosFlowList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AssociatedQosFlowItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowPerTNLInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMultiConnectivityMinusOne, []QosFlowPerTNLInformationItem(l))
}

func (l *QosFlowPerTNLInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowPerTNLInformationItem](r, enc, 1, maxnoofMultiConnectivityMinusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l UPTransportLayerInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMultiConnectivityMinusOne, []UPTransportLayerInformationItem(l))
}

func (l *UPTransportLayerInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UPTransportLayerInformationItem](r, enc, 1, maxnoofMultiConnectivityMinusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l UPTransportLayerInformationPairList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMultiConnectivityMinusOne, []UPTransportLayerInformationPairItem(l))
}

func (l *UPTransportLayerInformationPairList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UPTransportLayerInformationPairItem](r, enc, 1, maxnoofMultiConnectivityMinusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l ULNGUUPTNLModifyList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMultiConnectivity, []ULNGUUPTNLModifyItem(l))
}

func (l *ULNGUUPTNLModifyList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ULNGUUPTNLModifyItem](r, enc, 1, maxnoofMultiConnectivity)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowModifyConfirmList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowModifyConfirmItem(l))
}

func (l *QosFlowModifyConfirmList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowModifyConfirmItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowAddOrModifyRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowAddOrModifyRequestItem(l))
}

func (l *QosFlowAddOrModifyRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowAddOrModifyRequestItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowAcceptedList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowAcceptedItem(l))
}

func (l *QosFlowAcceptedList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowAcceptedItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowToBeForwardedList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowToBeForwardedItem(l))
}

func (l *QosFlowToBeForwardedList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowToBeForwardedItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowListWithDataForwarding) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowItemWithDataForwarding(l))
}

func (l *QosFlowListWithDataForwarding) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowItemWithDataForwarding](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l DataForwardingResponseDRBList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofDRBs, []DataForwardingResponseDRBItem(l))
}

func (l *DataForwardingResponseDRBList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[DataForwardingResponseDRBItem](r, enc, 1, maxnoofDRBs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceListHORqd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceItemHORqd(l))
}

func (l *PDUSessionResourceListHORqd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceItemHORqd](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceHandoverList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceHandoverItem(l))
}

func (l *PDUSessionResourceHandoverList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceHandoverItem](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceSetupListHOReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceSetupItemHOReq(l))
}

func (l *PDUSessionResourceSetupListHOReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceSetupItemHOReq](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceAdmittedList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceAdmittedItem(l))
}

func (l *PDUSessionResourceAdmittedList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceAdmittedItem](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToSetupListHOAck) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToSetupItemHOAck(l))
}

func (l *PDUSessionResourceFailedToSetupListHOAck) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToSetupItemHOAck](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceToBeSwitchedDLList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceToBeSwitchedDLItem(l))
}

func (l *PDUSessionResourceToBeSwitchedDLList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceToBeSwitchedDLItem](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceFailedToSetupListPSReq) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceFailedToSetupItemPSReq(l))
}

func (l *PDUSessionResourceFailedToSetupListPSReq) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceFailedToSetupItemPSReq](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceSwitchedList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceSwitchedItem(l))
}

func (l *PDUSessionResourceSwitchedList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceSwitchedItem](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceReleasedListPSAck) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceReleasedItemPSAck(l))
}

func (l *PDUSessionResourceReleasedListPSAck) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceReleasedItemPSAck](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceReleasedListPSFail) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceReleasedItemPSFail(l))
}

func (l *PDUSessionResourceReleasedListPSFail) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceReleasedItemPSFail](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l UEPresenceInAreaOfInterestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAoI, []UEPresenceInAreaOfInterestItem(l))
}

func (l *UEPresenceInAreaOfInterestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UEPresenceInAreaOfInterestItem](r, enc, 1, maxnoofAoI)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l PDUSessionResourceToReleaseListHOCmd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPDUSessions, []PDUSessionResourceToReleaseItemHOCmd(l))
}

func (l *PDUSessionResourceToReleaseListHOCmd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PDUSessionResourceToReleaseItemHOCmd](r, enc, 1, maxnoofPDUSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (l QosFlowNotifyList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowNotifyItem(l))
}

func (l *QosFlowNotifyList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowNotifyItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}
