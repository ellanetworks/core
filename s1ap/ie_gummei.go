// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/s1ap/aper"

// maxnoof bounds for ServedGUMMEIs and its lists (TS 36.413 §9.3).
const (
	maxnoofRATs        = 8
	maxnoofPLMNsPerMME = 32
	maxnoofGroupIDs    = 65535
	maxnoofMMECs       = 256
)

// MMEGroupID ::= OCTET STRING (SIZE(2)).
type MMEGroupID [2]byte

func (g MMEGroupID) encode(w *aper.Writer) error {
	return w.WriteOctetString(g[:], 2, 2, false)
}

func decodeMMEGroupID(r *aper.Reader) (MMEGroupID, error) {
	b, err := r.ReadOctetString(2, 2, false)
	if err != nil {
		return MMEGroupID{}, err
	}

	var g MMEGroupID

	copy(g[:], b)

	return g, nil
}

// MMECode ::= OCTET STRING (SIZE(1)).
type MMECode byte

func (c MMECode) encode(w *aper.Writer) error {
	return w.WriteOctetString([]byte{byte(c)}, 1, 1, false)
}

func decodeMMECode(r *aper.Reader) (MMECode, error) {
	b, err := r.ReadOctetString(1, 1, false)
	if err != nil {
		return 0, err
	}

	return MMECode(b[0]), nil
}

// GUMMEI ::= SEQUENCE { pLMN-Identity, mME-Group-ID, mME-Code, iE-Extensions
// OPTIONAL } (extensible), TS 36.413 §9.2.3.9. Identifies the MME an eNB
// selected; carried in INITIAL UE MESSAGE when the eNB does not run NNSF.
type GUMMEI struct {
	PLMNIdentity PLMNIdentity
	MMEGroupID   MMEGroupID
	MMECode      MMECode
}

func (g GUMMEI) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := g.PLMNIdentity.encode(w); err != nil {
		return err
	}

	if err := g.MMEGroupID.encode(w); err != nil {
		return err
	}

	return g.MMECode.encode(w)
}

func decodeGUMMEI(r *aper.Reader) (GUMMEI, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return GUMMEI{}, err
	}

	var g GUMMEI

	if g.PLMNIdentity, err = decodePLMNIdentity(r); err != nil {
		return g, err
	}

	if g.MMEGroupID, err = decodeMMEGroupID(r); err != nil {
		return g, err
	}

	if g.MMECode, err = decodeMMECode(r); err != nil {
		return g, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return g, err
	}

	return g, nil
}

// ServedGUMMEIsItem ::= SEQUENCE { servedPLMNs, servedGroupIDs, servedMMECs,
// iE-Extensions OPTIONAL } (extensible).
type ServedGUMMEIsItem struct {
	ServedPLMNs    []PLMNIdentity
	ServedGroupIDs []MMEGroupID
	ServedMMECs    []MMECode
}

func (it ServedGUMMEIsItem) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteConstrainedLength(len(it.ServedPLMNs), 1, maxnoofPLMNsPerMME); err != nil {
		return err
	}

	for _, p := range it.ServedPLMNs {
		if err := p.encode(w); err != nil {
			return err
		}
	}

	if err := w.WriteConstrainedLength(len(it.ServedGroupIDs), 1, maxnoofGroupIDs); err != nil {
		return err
	}

	for _, g := range it.ServedGroupIDs {
		if err := g.encode(w); err != nil {
			return err
		}
	}

	if err := w.WriteConstrainedLength(len(it.ServedMMECs), 1, maxnoofMMECs); err != nil {
		return err
	}

	for _, c := range it.ServedMMECs {
		if err := c.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeServedGUMMEIsItem(r *aper.Reader) (ServedGUMMEIsItem, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ServedGUMMEIsItem{}, err
	}

	var it ServedGUMMEIsItem

	nPLMN, err := r.ReadConstrainedLength(1, maxnoofPLMNsPerMME)
	if err != nil {
		return it, err
	}

	for i := 0; i < nPLMN; i++ {
		p, err := decodePLMNIdentity(r)
		if err != nil {
			return it, err
		}

		it.ServedPLMNs = append(it.ServedPLMNs, p)
	}

	nGroup, err := r.ReadConstrainedLength(1, maxnoofGroupIDs)
	if err != nil {
		return it, err
	}

	for i := 0; i < nGroup; i++ {
		g, err := decodeMMEGroupID(r)
		if err != nil {
			return it, err
		}

		it.ServedGroupIDs = append(it.ServedGroupIDs, g)
	}

	nMMEC, err := r.ReadConstrainedLength(1, maxnoofMMECs)
	if err != nil {
		return it, err
	}

	for i := 0; i < nMMEC; i++ {
		c, err := decodeMMECode(r)
		if err != nil {
			return it, err
		}

		it.ServedMMECs = append(it.ServedMMECs, c)
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ServedGUMMEIs ::= SEQUENCE (SIZE(1..maxnoofRATs)) OF ServedGUMMEIsItem.
type ServedGUMMEIs []ServedGUMMEIsItem

func (s ServedGUMMEIs) encode(w *aper.Writer) error {
	if err := w.WriteConstrainedLength(len(s), 1, maxnoofRATs); err != nil {
		return err
	}

	for _, it := range s {
		if err := it.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeServedGUMMEIs(r *aper.Reader) (ServedGUMMEIs, error) {
	n, err := r.ReadConstrainedLength(1, maxnoofRATs)
	if err != nil {
		return nil, err
	}

	out := make(ServedGUMMEIs, 0, n)

	for i := 0; i < n; i++ {
		it, err := decodeServedGUMMEIsItem(r)
		if err != nil {
			return nil, err
		}

		out = append(out, it)
	}

	return out, nil
}
