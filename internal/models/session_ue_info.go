// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"
	"time"
)

type RATType string

const (
	RATTypeNR      RATType = "NR"
	RATTypeEUTRA   RATType = "EUTRA"
	RATTypeVirtual RATType = "VIRTUAL"
)

func RATTypeOf(loc UserLocation) RATType {
	switch {
	case loc.NrLocation != nil:
		return RATTypeNR
	case loc.EutraLocation != nil:
		return RATTypeEUTRA
	case loc.N3gaLocation != nil:
		return RATTypeVirtual
	default:
		return ""
	}
}

type ServingNodeFunctionality string

const (
	ServingNodeAMF ServingNodeFunctionality = "AMF"
	ServingNodeSGW ServingNodeFunctionality = "SGW"
)

type ServingNode struct {
	Functionality ServingNodeFunctionality
	GUAMI         *Guami
}

type SessionUEInfo struct {
	PEI            string
	Location       UserLocation
	RATType        RATType
	TimeZone       string
	ServingNode    ServingNode
	ServingNetwork PlmnID
}

type SessionUEInfoUpdate struct {
	PEI            *string
	Location       *UserLocation
	RATType        *RATType
	TimeZone       *string
	ServingNode    *ServingNode
	ServingNetwork *PlmnID
}

func (u SessionUEInfoUpdate) Empty() bool {
	return u.PEI == nil && u.Location == nil && u.RATType == nil && u.TimeZone == nil &&
		u.ServingNode == nil && u.ServingNetwork == nil
}

func (u SessionUEInfoUpdate) ApplyTo(info *SessionUEInfo) {
	if u.PEI != nil {
		info.PEI = *u.PEI
	}

	if u.Location != nil {
		info.Location = *u.Location
	}

	if u.RATType != nil {
		info.RATType = *u.RATType
	}

	if u.TimeZone != nil {
		info.TimeZone = *u.TimeZone
	}

	if u.ServingNode != nil {
		info.ServingNode = *u.ServingNode
	}

	if u.ServingNetwork != nil {
		info.ServingNetwork = *u.ServingNetwork
	}
}

func LocationUpdate(loc UserLocation, now time.Time) SessionUEInfoUpdate {
	var update SessionUEInfoUpdate

	if loc.EutraLocation != nil || loc.NrLocation != nil || loc.N3gaLocation != nil {
		rat := RATTypeOf(loc)
		update.Location = &loc
		update.RATType = &rat
	}

	tz := LocalTimeZone(now)
	update.TimeZone = &tz

	return update
}

func (l UserLocation) SameCell(o UserLocation) bool {
	switch {
	case l.NrLocation != nil && o.NrLocation != nil:
		return sameTai(l.NrLocation.Tai, o.NrLocation.Tai) && sameNcgi(l.NrLocation.Ncgi, o.NrLocation.Ncgi)
	case l.EutraLocation != nil && o.EutraLocation != nil:
		return sameTai(l.EutraLocation.Tai, o.EutraLocation.Tai) && sameEcgi(l.EutraLocation.Ecgi, o.EutraLocation.Ecgi)
	case l.N3gaLocation != nil && o.N3gaLocation != nil:
		return sameTai(l.N3gaLocation.N3gppTai, o.N3gaLocation.N3gppTai)
	default:
		return l.NrLocation == nil && o.NrLocation == nil &&
			l.EutraLocation == nil && o.EutraLocation == nil &&
			l.N3gaLocation == nil && o.N3gaLocation == nil
	}
}

func sameTai(a, b *Tai) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Equal(*b)
}

func samePlmn(a, b *PlmnID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Equal(*b)
}

func sameNcgi(a, b *Ncgi) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.NrCellID == b.NrCellID && samePlmn(a.PlmnID, b.PlmnID)
}

func sameEcgi(a, b *Ecgi) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.EutraCellID == b.EutraCellID && samePlmn(a.PlmnID, b.PlmnID)
}

func LocalTimeZone(now time.Time) string {
	_, offset := now.Zone()
	if now.IsDST() {
		offset -= 3600
	}

	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}

	tz := fmt.Sprintf("%s%02d:%02d", sign, offset/3600, (offset%3600)/60)
	if now.IsDST() {
		tz += "+1"
	}

	return tz
}

type ChargingCharacteristicsSelectionMode string

const (
	ChargingCharacteristicsSubscribed  ChargingCharacteristicsSelectionMode = ""
	ChargingCharacteristicsHomeDefault ChargingCharacteristicsSelectionMode = "HOME_DEFAULT"
)

type ChargingCharacteristics struct {
	Value         string
	SelectionMode ChargingCharacteristicsSelectionMode
}

var DefaultChargingCharacteristics = ChargingCharacteristics{Value: "0800", SelectionMode: ChargingCharacteristicsHomeDefault}
