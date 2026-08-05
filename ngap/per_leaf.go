// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// pergen loads this package through the root module, which therefore requires
// and replaces it.
//go:generate sh -c "cd .. && go run ./cmd/pergen -o ngap/per_gen.go github.com/ellanetworks/core/ngap"

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

// userLocationCellIDBits is the CGI cell-identity width for each kind, and the
// CHOICE alternative that selects it.
var userLocationCellIDBits = map[UserLocationInformationKind]struct {
	alt  int64
	bits int
}{
	UserLocationEUTRA: {userLocationInformationEUTRA, EUTRACellIdentityBits},
	UserLocationNR:    {userLocationInformationNR, NRCellIdentityBits},
}

func (u UserLocationInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if u.Kind == UserLocationN3IWF {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, userLocationInformationAlternatives-1, userLocationInformationN3IWF); err != nil {
			return err
		}

		// SEQUENCE { iPAddress, portNumber, iE-Extensions OPTIONAL, ... }:
		// extension bit, then the one OPTIONAL field's presence bit.
		w.WriteBit(false)
		w.WriteBit(false)

		if err := u.IPAddress.MarshalPER(w, enc); err != nil {
			return err
		}

		return per.EncodeOctetString(w, enc, 2, 2, true, true, false,
			[]byte{byte(u.PortNumber >> 8), byte(u.PortNumber)})
	}

	shape, ok := userLocationCellIDBits[u.Kind]
	if !ok {
		return fmt.Errorf("ngap: invalid UserLocationInformation kind %d", u.Kind)
	}

	if shape.bits < 64 && u.CellIdentity >= 1<<uint(shape.bits) {
		return fmt.Errorf("ngap: cell identity %d exceeds the %d-bit field of kind %d",
			u.CellIdentity, shape.bits, u.Kind)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, userLocationInformationAlternatives-1, shape.alt); err != nil {
		return err
	}

	// UserLocationInformationXxx ::= SEQUENCE { <CGI>, tAI, timeStamp OPTIONAL,
	// iE-Extensions OPTIONAL, ... }: extension bit, then a presence bit each for
	// timeStamp and iE-Extensions.
	w.WriteBit(false)
	w.WriteBit(u.TimeStamp != nil)
	w.WriteBit(false)

	// Xxx-CGI ::= SEQUENCE { pLMNIdentity, <cell identity>, iE-Extensions
	// OPTIONAL, ... }: its own extension bit and presence bit.
	w.WriteBit(false)
	w.WriteBit(false)

	if err := u.PLMNIdentity.MarshalPER(w, enc); err != nil {
		return err
	}

	if err := encodeBitStringUint(w, enc, u.CellIdentity, shape.bits); err != nil {
		return err
	}

	if err := u.TAI.MarshalPER(w, enc); err != nil {
		return err
	}

	if u.TimeStamp != nil {
		return per.EncodeOctetString(w, enc, 4, 4, true, true, false, u.TimeStamp[:])
	}

	return nil
}

func (u *UserLocationInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	alt, err := per.DecodeConstrainedWholeNumber(r, enc, 0, userLocationInformationAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: UserLocationInformation choice: %w", err)
	}

	var kind UserLocationInformationKind

	switch alt {
	case userLocationInformationEUTRA:
		kind = UserLocationEUTRA
	case userLocationInformationNR:
		kind = UserLocationNR
	case userLocationInformationN3IWF:
		return u.unmarshalN3IWF(r, enc)
	case userLocationInformationChoiceExtensions:
		return decodeChoiceExtension(r, enc, "UserLocationInformation")
	default:
		// Leave the value untouched: a zero location must not read as one the
		// peer reported.
		return fmt.Errorf("%w: UserLocationInformation alternative %d", errNotComprehended, alt)
	}

	bits := userLocationCellIDBits[kind].bits

	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	hasTimeStamp, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	cgiExtBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	cgiExtContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	var out UserLocationInformation

	out.Kind = kind

	if err := out.PLMNIdentity.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if out.CellIdentity, err = decodeBitStringUint(r, enc, bits); err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, cgiExtContainer, cgiExtBit); err != nil {
		return err
	}

	if err := out.TAI.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if hasTimeStamp {
		b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
		if err != nil {
			return err
		}

		var ts TimeStamp

		copy(ts[:], b)

		out.TimeStamp = &ts
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*u = out

	return nil
}

func (u *UserLocationInformation) unmarshalN3IWF(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	var out UserLocationInformation

	out.Kind = UserLocationN3IWF

	if err := out.IPAddress.UnmarshalPER(r, enc); err != nil {
		return err
	}

	port, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	out.PortNumber = PortNumber(uint16(port[0])<<8 | uint16(port[1]))

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*u = out

	return nil
}

func (g GlobalRANNodeID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	shape, ok := ranNodeIDShapes[g.Kind]
	if !ok {
		return fmt.Errorf("ngap: invalid GlobalRANNodeID kind %d", g.Kind)
	}

	if g.Bits < shape.lb || g.Bits > shape.ub {
		return fmt.Errorf("ngap: GlobalRANNodeID kind %d: %d bits outside SIZE(%d..%d)",
			g.Kind, g.Bits, shape.lb, shape.ub)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, globalRANNodeIDAlternatives-1, int64(shape.outer)); err != nil {
		return err
	}

	// Global*-ID ::= SEQUENCE { pLMNIdentity, <node id>, iE-Extensions
	// OPTIONAL, ... }: extension bit, then the OPTIONAL field's presence bit.
	w.WriteBit(false)
	w.WriteBit(false)

	if err := g.PLMNIdentity.MarshalPER(w, enc); err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, int64(nodeIDAlternatives[shape.outer]-1), int64(shape.inner)); err != nil {
		return err
	}

	return per.EncodeBitString(w, enc, int64(shape.lb), int64(shape.ub), true, true, false,
		uintToBits(uint64(g.Value), g.Bits), g.Bits)
}

func (g *GlobalRANNodeID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	outer, err := per.DecodeConstrainedWholeNumber(r, enc, 0, globalRANNodeIDAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: GlobalRANNodeID choice: %w", err)
	}

	if outer == globalRANNodeIDChoiceExtensions {
		return decodeChoiceExtension(r, enc, "GlobalRANNodeID")
	}

	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	var plmn PLMNIdentity
	if err := plmn.UnmarshalPER(r, enc); err != nil {
		return err
	}

	alternatives := nodeIDAlternatives[int(outer)]

	inner, err := per.DecodeConstrainedWholeNumber(r, enc, 0, int64(alternatives-1))
	if err != nil {
		return fmt.Errorf("ngap: %s choice: %w", nodeIDChoiceName[int(outer)], err)
	}

	// The last alternative of each nested CHOICE is its choice-Extensions.
	if int(inner) == alternatives-1 {
		return decodeChoiceExtension(r, enc, nodeIDChoiceName[int(outer)])
	}

	kind, ok := kindForShape[[2]int{int(outer), int(inner)}]
	if !ok {
		return fmt.Errorf("ngap: unreachable GlobalRANNodeID alternative %d/%d", outer, inner)
	}

	shape := ranNodeIDShapes[kind]

	b, nbits, err := per.DecodeBitString(r, enc, int64(shape.lb), int64(shape.ub), true, true, false)
	if err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*g = GlobalRANNodeID{
		Kind:         kind,
		PLMNIdentity: plmn,
		Value:        uint32(bitsToUint(b, nbits)),
		Bits:         nbits,
	}

	return nil
}

func (u UERetentionInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, ueRetentionInformationRootCount, int64(u), "UERetentionInformation")
}

func (u *UERetentionInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, ueRetentionInformationRootCount, "UERetentionInformation")
	if err != nil {
		return err
	}

	*u = UERetentionInformation(idx)

	return nil
}

func (p PagingDRX) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, pagingDRXRootCount, int64(p), "PagingDRX")
}

func (p *PagingDRX) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, pagingDRXRootCount, "PagingDRX")
	if err != nil {
		return err
	}

	*p = PagingDRX(idx)

	return nil
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

// A ProtocolExtensionContainer: never encoded, and on decode only its
// criticality is acted on.
type ieExtensions struct{}

func (ieExtensions) MarshalPER(*per.Writer, per.Encoding) error {
	return fmt.Errorf("ngap: unmodeled iE-Extensions cannot be encoded")
}

func (*ieExtensions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, maxProtocolExtensions)
	if err != nil {
		return err
	}

	var rejected ProtocolIEID

	comprehended := true

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return err
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return err
		}

		if err := per.SkipOpenType(r, enc); err != nil {
			return err
		}

		// §10.3.4.2: an unmodeled extension marked reject stops the procedure;
		// ignore and notify are skipped and the IE holding them still
		// delivered. The container is consumed either way so the reader stays
		// aligned. A notify extension should also be reported back, which a
		// leaf decoder has no diagnostics sink to do.
		if Criticality(crit) == CriticalityReject && comprehended {
			comprehended, rejected = false, ProtocolIEID(id)
		}
	}

	if !comprehended {
		return &notComprehendedIE{ID: rejected, Crit: CriticalityReject, What: "iE-Extensions"}
	}

	return nil
}

func skipSequenceExtensionsPER(r *per.Reader, enc per.Encoding, extContainer, extAdditions bool) error {
	if extContainer {
		var e ieExtensions
		if err := e.UnmarshalPER(r, enc); err != nil {
			return err
		}
	}

	if !extAdditions {
		return nil
	}

	var present []bool

	// A bitmap wider than 64 bits arrives fragmented (X.691 §19.7 via §11.9.3),
	// so the callback runs once per fragment and the bits accumulate.
	err := per.DecodeNormallySmallLength(r, enc, func(count int64) error {
		for range count {
			b, err := r.ReadBit()
			if err != nil {
				return err
			}

			present = append(present, b)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, p := range present {
		if p {
			if err := per.SkipOpenType(r, enc); err != nil {
				return err
			}
		}
	}

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

func perIEDecode(b []byte, u per.Unmarshaler) error {
	return u.UnmarshalPER(per.NewReader(b), per.Aligned)
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
