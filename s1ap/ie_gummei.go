// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per"

// maxnoof bounds for ServedGUMMEIs and its lists (TS 36.413).
const (
	maxnoofRATs        = 8
	maxnoofPLMNsPerMME = 32
	maxnoofGroupIDs    = 65535
	maxnoofMMECs       = 256
)

// MMEGroupID ::= OCTET STRING (SIZE(2)).
type MMEGroupID [2]byte

func (g MMEGroupID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, g[:])
}

func (g *MMEGroupID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	copy(g[:], b)

	return nil
}

// MMECode ::= OCTET STRING (SIZE(1)).
type MMECode byte

func (c MMECode) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 1, true, true, false, []byte{byte(c)})
}

func (c *MMECode) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 1, true, true, false)
	if err != nil {
		return err
	}

	*c = MMECode(b[0])

	return nil
}

// GUMMEI ::= SEQUENCE { pLMN-Identity, mME-Group-ID, mME-Code, iE-Extensions
// OPTIONAL } (extensible), TS 36.413. Identifies the MME an eNB
// selected; carried in INITIAL UE MESSAGE when the eNB does not run NNSF.
type GUMMEI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	MMEGroupID   MMEGroupID
	MMECode      MMECode
	_            ieExtensions `per:",skip"`
}

// ServedGUMMEIsItem ::= SEQUENCE { servedPLMNs, servedGroupIDs, servedMMECs,
// iE-Extensions OPTIONAL } (extensible).
type ServedGUMMEIsItem struct {
	_              [0]struct{}    `per:"extseq"`
	ServedPLMNs    []PLMNIdentity `per:"SEQUENCE-OF,size:1..32"`
	ServedGroupIDs []MMEGroupID   `per:"SEQUENCE-OF,size:1..65535"`
	ServedMMECs    []MMECode      `per:"SEQUENCE-OF,size:1..256"`
	_              ieExtensions   `per:",skip"`
}

// ServedGUMMEIs ::= SEQUENCE (SIZE(1..maxnoofRATs)) OF ServedGUMMEIsItem.
type ServedGUMMEIs []ServedGUMMEIsItem

func (s ServedGUMMEIs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofRATs, []ServedGUMMEIsItem(s))
}

func (s *ServedGUMMEIs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ServedGUMMEIsItem](r, enc, 1, maxnoofRATs)
	if err != nil {
		return err
	}

	*s = items

	return nil
}
