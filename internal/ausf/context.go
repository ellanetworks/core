// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"regexp"
	"sync"

	"github.com/ellanetworks/core/internal/models"
)

type AUSFContext struct {
	suciSupiMap sync.Map
	UePool      sync.Map
	snRegex     *regexp.Regexp
}

type AusfUeContext struct {
	Supi               string
	Kausf              string
	Kseaf              string
	ServingNetworkName string
	AuthStatus         models.AuthResult

	// for 5G AKA
	XresStar string

	// for EAP-AKA'
	kAut string
	XRES string
	Rand string
}

type SuciSupiMap struct {
	SupiOrSuci string
	Supi       string
}

const (
	EapAkaPrimeTypeNum = 50
)

// Attribute Types for EAP-AKA'
const (
	AtRandAttribute         = 1
	AtAutnAttribute         = 2
	AtResAttribute          = 3
	AtMacAttribute          = 11
	AtNotificationAttribute = 12
	AtIdentityAttribute     = 14
	AtKdfInputAttribute     = 23
	AtKdfAttribute          = 24
)

var ausfContext AUSFContext

func NewAusfUeContext(identifier string) (ausfUeContext *AusfUeContext) {
	ausfUeContext = new(AusfUeContext)
	ausfUeContext.Supi = identifier
	return ausfUeContext
}

func AddAusfUeContextToPool(ausfUeContext *AusfUeContext) {
	ausfContext.UePool.Store(ausfUeContext.Supi, ausfUeContext)
}

func CheckIfAusfUeContextExists(ref string) bool {
	_, ok := ausfContext.UePool.Load(ref)
	return ok
}

func GetAusfUeContext(ref string) *AusfUeContext {
	context, _ := ausfContext.UePool.Load(ref)
	ausfUeContext := context.(*AusfUeContext)
	return ausfUeContext
}

func AddSuciSupiPairToMap(supiOrSuci string, supi string) {
	newPair := new(SuciSupiMap)
	newPair.SupiOrSuci = supiOrSuci
	newPair.Supi = supi
	ausfContext.suciSupiMap.Store(supiOrSuci, newPair)
}

func CheckIfSuciSupiPairExists(ref string) bool {
	_, ok := ausfContext.suciSupiMap.Load(ref)
	return ok
}

func GetSupiFromSuciSupiMap(ref string) string {
	val, _ := ausfContext.suciSupiMap.Load(ref)
	suciSupiMap := val.(*SuciSupiMap)
	supi := suciSupiMap.Supi
	return supi
}

func IsServingNetworkAuthorized(lookup string) bool {
	if ausfContext.snRegex.MatchString(lookup) {
		return true
	} else {
		return false
	}
}
