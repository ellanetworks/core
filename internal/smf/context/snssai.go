// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import "github.com/omec-project/openapi/models"

type SNssai struct {
	Sd  string
	Sst int32
}

// Equal return true if two S-NSSAI is equal
func (s *SNssai) Equal(target *SNssai) bool {
	return s.Sst == target.Sst && s.Sd == target.Sd
}

type SnssaiUPFInfo struct {
	SNssai  SNssai
	DnnList []DnnUPFInfoItem
}

// DnnUpfInfoItem presents UPF dnn information
type DnnUPFInfoItem struct {
	Dnn             string
	DnaiList        []string
	PduSessionTypes []models.PduSessionType
}

// ContainsDNAI return true if the this dnn Info contains the specify DNAI
func (d *DnnUPFInfoItem) ContainsDNAI(targetDnai string) bool {
	if targetDnai == "" {
		return d.DnaiList == nil || len(d.DnaiList) == 0
	}
	for _, dnai := range d.DnaiList {
		if dnai == targetDnai {
			return true
		}
	}
	return false
}
