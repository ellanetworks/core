// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
)

type cellKey struct {
	rat   RAT
	pci   int64
	arfcn int64
}

type CellCollector struct {
	source MeasurementSource
	cells  []*CellMeasurement
	index  map[cellKey]*CellMeasurement
}

func NewCellCollector(source MeasurementSource) *CellCollector {
	return &CellCollector{source: source, index: map[cellKey]*CellMeasurement{}}
}

func (c *CellCollector) Cell(rat RAT, pci, arfcn int64) *CellMeasurement {
	key := cellKey{rat: rat, pci: pci, arfcn: arfcn}
	if cell, ok := c.index[key]; ok {
		return cell
	}

	cell := &CellMeasurement{Source: c.source, RAT: rat, PCI: &pci, ARFCN: &arfcn}
	c.index[key] = cell
	c.cells = append(c.cells, cell)

	return cell
}

func (c *CellCollector) Serving(rat RAT, ncgi *models.Ncgi, ecgi *models.Ecgi) *CellMeasurement {
	for _, cell := range c.cells {
		if cell.RAT != rat {
			continue
		}

		if (ncgi != nil && SameNcgi(cell.NCGI, ncgi)) || (ecgi != nil && SameEcgi(cell.ECGI, ecgi)) {
			cell.Serving = true
			return cell
		}
	}

	cell := &CellMeasurement{Source: c.source, RAT: rat, Serving: true, NCGI: ncgi, ECGI: ecgi}
	c.cells = append(c.cells, cell)

	return cell
}

func (c *CellCollector) Cells() []CellMeasurement {
	if len(c.cells) == 0 {
		return nil
	}

	out := make([]CellMeasurement, len(c.cells))
	for i, cell := range c.cells {
		out[i] = *cell
	}

	return out
}

func Beam(beams *[]BeamMeasurement, index int64) *BeamMeasurement {
	for i := range *beams {
		if (*beams)[i].Index == index {
			return &(*beams)[i]
		}
	}

	*beams = append(*beams, BeamMeasurement{Index: index})

	return &(*beams)[len(*beams)-1]
}

func PlmnFromOctets(b []byte) (*models.PlmnID, error) {
	if len(b) != 3 {
		return nil, fmt.Errorf("PLMN identity is %d octets, want 3", len(b))
	}

	p, err := nas.ParsePLMN([3]byte(b))
	if err != nil {
		return nil, err
	}

	return &models.PlmnID{Mcc: p.MCC, Mnc: p.MNC}, nil
}

func NewNcgi(plmn *models.PlmnID, cellID uint64) *models.Ncgi {
	return &models.Ncgi{PlmnID: plmn, NrCellID: fmt.Sprintf("%09x", cellID)}
}

func NewEcgi(plmn *models.PlmnID, cellID uint64) *models.Ecgi {
	return &models.Ecgi{PlmnID: plmn, EutraCellID: fmt.Sprintf("%07x", cellID)}
}

func SameNcgi(a, b *models.Ncgi) bool {
	return a != nil && b != nil && a.NrCellID == b.NrCellID && samePlmn(a.PlmnID, b.PlmnID)
}

func SameEcgi(a, b *models.Ecgi) bool {
	return a != nil && b != nil && a.EutraCellID == b.EutraCellID && samePlmn(a.PlmnID, b.PlmnID)
}

func samePlmn(a, b *models.PlmnID) bool {
	return a != nil && b != nil && a.Equal(*b)
}
