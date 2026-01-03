// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"regexp"
	"sync"

	"github.com/ellanetworks/core/internal/db"
)

type AUSFContext struct {
	Mutex sync.Mutex

	UePool     map[string]*AusfUeContext // Key: suci
	DBInstance *db.Database
}

type AusfUeContext struct {
	Supi  string
	Kseaf string

	// for 5G AKA
	XresStar string
	Rand     string
}

var ausfContext AUSFContext

var servingNetworkRegex = regexp.MustCompile(`^5G:mnc[0-9]{3}\.mcc[0-9]{3}\.3gppnetwork\.org$`)

func addUeContextToPool(suci string, ausfUeContext *AusfUeContext) {
	ausfContext.Mutex.Lock()
	defer ausfContext.Mutex.Unlock()

	ausfContext.UePool[suci] = ausfUeContext
}

func getUeContext(suci string) *AusfUeContext {
	ausfContext.Mutex.Lock()
	defer ausfContext.Mutex.Unlock()

	ausfUeContext, ok := ausfContext.UePool[suci]
	if !ok {
		return nil
	}

	return ausfUeContext
}

func isServingNetworkAuthorized(lookup string) bool {
	return servingNetworkRegex.MatchString(lookup)
}

func Start(dbInstance *db.Database) {
	ausfContext = AUSFContext{
		UePool:     make(map[string]*AusfUeContext),
		DBInstance: dbInstance,
	}
}
