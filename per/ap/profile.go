// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import (
	"fmt"
	"slices"
)

const maxContainerBound = 65535

type Profile struct {
	Name                  string
	MaxProtocolIEs        int64
	MaxProtocolExtensions int64
	OpaqueProcedures      []uint8
	ProcedureName         func(procedureCode uint8) string
	IEName                func(id uint16) string
}

func (p *Profile) validate() error {
	switch {
	case p == nil:
		return fmt.Errorf("%w: nil", ErrProfile)
	case p.Name == "":
		return fmt.Errorf("%w: no name", ErrProfile)
	case p.MaxProtocolIEs < 1 || p.MaxProtocolIEs > maxContainerBound:
		return fmt.Errorf("%w: %s: maxProtocolIEs %d", ErrProfile, p.Name, p.MaxProtocolIEs)
	case p.MaxProtocolExtensions < 1 || p.MaxProtocolExtensions > maxContainerBound:
		return fmt.Errorf("%w: %s: maxProtocolExtensions %d", ErrProfile, p.Name, p.MaxProtocolExtensions)
	default:
		return nil
	}
}

func (p *Profile) Opaque(procedureCode uint8) bool {
	return slices.Contains(p.OpaqueProcedures, procedureCode)
}

func (p *Profile) procedureName(procedureCode uint8) string {
	if p.ProcedureName != nil {
		return p.ProcedureName(procedureCode)
	}

	return fmt.Sprintf("procedure %d", procedureCode)
}

func (p *Profile) ieName(id uint16) string {
	if p.IEName != nil {
		return p.IEName(id)
	}

	return fmt.Sprintf("%d", id)
}

func (p *Profile) errorf(format string, args ...any) error {
	return fmt.Errorf(p.Name+": "+format, args...)
}
