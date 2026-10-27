// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"errors"
	"regexp"
	"strings"

	"github.com/ellanetworks/core/internal/models"
)

var amfIDPattern = regexp.MustCompile(`^[A-Fa-f0-9]{6}$`)

type Guami struct {
	PlmnID *PlmnIDNid `json:"plmnId"`
	AmfID  string     `json:"amfId"`
}

func (g Guami) Check(p Policy) error {
	return firstError(
		CheckRequired("/plmnId", g.PlmnID, p),
		checkPattern("/amfId", g.AmfID, amfIDPattern, false),
	)
}

func (g Guami) Equal(o Guami) bool {
	return g.PlmnID != nil && o.PlmnID != nil && *g.PlmnID == *o.PlmnID && strings.EqualFold(g.AmfID, o.AmfID)
}

func GuamiFromModel(m models.Guami) (Guami, error) {
	if m.PlmnID == nil {
		return Guami{}, errors.New("wire: GUAMI without PLMN")
	}

	return Guami{PlmnID: &PlmnIDNid{Mcc: m.PlmnID.Mcc, Mnc: m.PlmnID.Mnc}, AmfID: strings.ToLower(m.AmfID)}, nil
}

func (g Guami) Model() (models.Guami, error) {
	if err := g.Check(Strict); err != nil {
		return models.Guami{}, err
	}

	plmn := g.PlmnID.PlmnID().Model()

	return models.Guami{PlmnID: &plmn, AmfID: strings.ToLower(g.AmfID)}, nil
}
