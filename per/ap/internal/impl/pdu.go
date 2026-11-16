// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/per"
)

const (
	pduChoiceCount   = 3
	maxProcedureCode = 255
)

type Envelope struct {
	Choice        int
	ProcedureCode uint8
	Criticality   Criticality
	Value         []byte
}

func EncodePDU(p *Profile, e Envelope) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	w := per.NewWriter()
	enc := per.Aligned

	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, pduChoiceCount-1, int64(e.Choice)); err != nil {
		return nil, p.errorf("PDU choice: %w", err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, maxProcedureCode, int64(e.ProcedureCode)); err != nil {
		return nil, p.errorf("procedureCode: %w", err)
	}

	if err := per.EncodeEnumerated(w, enc, criticalityCount, false, int64(e.Criticality)); err != nil {
		return nil, p.errorf("criticality: %w", err)
	}

	if err := per.EncodeOpenTypeBytes(w, enc, e.Value); err != nil {
		return nil, p.errorf("value: %w", err)
	}

	w.AlignToByte()

	return w.Bytes(), nil
}

func DecodePDU[P ~uint8](p *Profile, b []byte) (Envelope, error) {
	if err := p.validate(); err != nil {
		return Envelope{}, err
	}

	r := per.NewReader(b)

	e, err := decodeHeader[P](p, r)
	if err != nil {
		return Envelope{}, err
	}

	val, err := per.DecodeOpenTypeBytes(r, per.Aligned)
	if err == nil {
		err = r.Complete()
	}

	if err != nil {
		return Envelope{}, procedureSyntaxError(p, P(e.ProcedureCode), fmt.Errorf("value: %w", err))
	}

	e.Value = val

	return e, nil
}

func envelopeSyntaxError[P ~uint8](p *Profile, err error) error {
	return &TransferSyntaxError[P]{Protocol: p.Name, Err: err}
}

func procedureSyntaxError[P ~uint8](p *Profile, procedure P, err error) error {
	return &TransferSyntaxError[P]{Protocol: p.Name, Procedure: procedure, ProcedureKnown: true, Err: err}
}

func decodeHeader[P ~uint8](p *Profile, r *per.Reader) (Envelope, error) {
	enc := per.Aligned

	ext, err := r.ReadBit()
	if err != nil {
		return Envelope{}, envelopeSyntaxError[P](p, fmt.Errorf("PDU choice: %w", err))
	}

	if ext {
		return Envelope{}, envelopeSyntaxError[P](p, extensionAlternativeError{protocol: p.Name})
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, pduChoiceCount-1)
	if err != nil {
		return Envelope{}, envelopeSyntaxError[P](p, fmt.Errorf("PDU choice: %w", err))
	}

	pc, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProcedureCode)
	if err != nil {
		return Envelope{}, envelopeSyntaxError[P](p, fmt.Errorf("procedureCode: %w", err))
	}

	crit, err := per.DecodeEnumerated(r, enc, criticalityCount, false)
	if err != nil {
		return Envelope{}, procedureSyntaxError(p, P(pc), fmt.Errorf("criticality: %w", err))
	}

	return Envelope{Choice: int(idx), ProcedureCode: uint8(pc), Criticality: Criticality(crit)}, nil
}

type extensionAlternativeError struct {
	protocol string
}

func (e extensionAlternativeError) Error() string {
	return fmt.Sprintf("%s: unsupported %s-PDU extension alternative", e.protocol, strings.ToUpper(e.protocol))
}

func (e extensionAlternativeError) Is(target error) bool { return target == ErrExtensionAlternative }
