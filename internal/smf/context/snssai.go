// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import "github.com/ellanetworks/core/internal/models"

type SNssai struct {
	Sd  string
	Sst int32
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
