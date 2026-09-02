// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"time"

	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
)

type GPRSTimer2Value struct {
	Unit        utils.EnumField `json:"unit"`
	Value       uint8           `json:"value"`
	Duration    string          `json:"duration"`
	Seconds     *int64          `json:"seconds,omitempty"`
	Deactivated bool            `json:"deactivated,omitempty"`
}

type GPRSTimer3Value struct {
	Unit        utils.EnumField `json:"unit"`
	Value       uint8           `json:"value"`
	Duration    string          `json:"duration"`
	Seconds     *int64          `json:"seconds,omitempty"`
	Deactivated bool            `json:"deactivated,omitempty"`
}

func gprsTimer3(t *naslib.GPRSTimer3) *GPRSTimer3Value {
	if t == nil {
		return nil
	}

	out := &GPRSTimer3Value{
		Unit:        utils.NamedEnum(t.Unit, t.UnitName()),
		Value:       t.Value,
		Duration:    t.String(),
		Deactivated: t.Deactivated(),
	}

	if d, ok := t.Duration(); ok {
		secs := int64(d / time.Second)
		out.Seconds = &secs
	}

	return out
}

func gprsTimer2(t *naslib.GPRSTimer2) *GPRSTimer2Value {
	if t == nil {
		return nil
	}

	out := &GPRSTimer2Value{
		Unit:        utils.NamedEnum(t.Unit, t.UnitName()),
		Value:       t.Value,
		Duration:    t.String(),
		Deactivated: t.Deactivated(),
	}

	if d, ok := t.Duration(); ok {
		secs := int64(d / time.Second)
		out.Seconds = &secs
	}

	return out
}
