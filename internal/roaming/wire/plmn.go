// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ellanetworks/core/internal/models"
)

var (
	mccPattern = regexp.MustCompile(`^\d{3}$`)
	mncPattern = regexp.MustCompile(`^\d{2,3}$`)
	nidPattern = regexp.MustCompile(`^[A-Fa-f0-9]{11}$`)
)

type PlmnID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

func PlmnIDFromModel(m models.PlmnID) PlmnID {
	return PlmnID{Mcc: m.Mcc, Mnc: m.Mnc}
}

func (p PlmnID) Model() models.PlmnID {
	return models.PlmnID{Mcc: p.Mcc, Mnc: p.Mnc}
}

func (p PlmnID) Check(Policy) error {
	return firstError(
		checkPattern("/mcc", p.Mcc, mccPattern, false),
		checkPattern("/mnc", p.Mnc, mncPattern, false),
	)
}

func (p PlmnID) Valid() bool {
	return p.Check(Strict) == nil
}

func (p PlmnID) String() string {
	return p.Mcc + "-" + p.Mnc
}

func ParsePlmnID(s string) (PlmnID, error) {
	mcc, mnc, ok := strings.Cut(s, "-")
	if !ok {
		return PlmnID{}, fmt.Errorf("wire: PLMN %q is not <mcc>-<mnc>", s)
	}

	p := PlmnID{Mcc: mcc, Mnc: mnc}
	if !p.Valid() {
		return PlmnID{}, fmt.Errorf("wire: PLMN %q is not <mcc>-<mnc>", s)
	}

	return p, nil
}

func (p PlmnID) mnc3() string {
	if len(p.Mnc) == 2 {
		return "0" + p.Mnc
	}

	return p.Mnc
}

func (p PlmnID) ServingNetworkName() string {
	return "5G:mnc" + p.mnc3() + ".mcc" + p.Mcc + ".3gppnetwork.org"
}

type PlmnIDNid struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
	Nid string `json:"nid,omitempty"`
}

func (p PlmnIDNid) PlmnID() PlmnID {
	return PlmnID{Mcc: p.Mcc, Mnc: p.Mnc}
}

func (p PlmnIDNid) Check(pol Policy) error {
	return firstError(
		p.PlmnID().Check(pol),
		checkPattern("/nid", p.Nid, nidPattern, true),
	)
}
