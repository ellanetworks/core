// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"regexp"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

type AUSFContext struct {
	suciSupiMap sync.Map
	UePool      sync.Map
	DBInstance  *db.Database
}

type AusfUeContext struct {
	Supi               string
	Kausf              string
	Kseaf              string
	ServingNetworkName string
	AuthStatus         models.AuthResult

	// for 5G AKA
	XresStar string
	XRES     string
	Rand     string
}

type SuciSupiMap struct {
	Suci string
	Supi string
}

var ausfContext AUSFContext

var servingNetworkRegex = regexp.MustCompile(`^5G:mnc[0-9]{3}\.mcc[0-9]{3}\.3gppnetwork\.org$`)

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
	context, ok := ausfContext.UePool.Load(ref)
	if !ok {
		return nil
	}
	ausfUeContext := context.(*AusfUeContext)
	return ausfUeContext
}

func AddSuciSupiPairToMap(suci string, supi string) {
	newPair := new(SuciSupiMap)
	newPair.Suci = suci
	newPair.Supi = supi
	ausfContext.suciSupiMap.Store(suci, newPair)
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
	return servingNetworkRegex.MatchString(lookup)
}

func SetDBInstance(dbInstance *db.Database) {
	ausfContext.DBInstance = dbInstance
}

func Start(dbInstance *db.Database) error {
	ausfContext.DBInstance = dbInstance
	return nil
}
