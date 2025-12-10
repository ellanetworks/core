// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"sync"

	"github.com/ellanetworks/core/internal/models"
)

type SmContext struct {
	Mu sync.Mutex // protect the following fields

	PduSessionIDVal       int32
	SmContextRefVal       string
	SnssaiVal             models.Snssai
	DnnVal                string
	UserLocationVal       models.UserLocation
	PduSessionInactiveVal bool
}

func NewSmContext(pduSessionID int32) *SmContext {
	c := &SmContext{
		PduSessionIDVal: pduSessionID,
	}
	return c
}

func (c *SmContext) IsPduSessionActive() bool {
	return !c.PduSessionInactiveVal
}

func (c *SmContext) SetPduSessionInActive(s bool) {
	c.PduSessionInactiveVal = s
}

func (c *SmContext) PduSessionID() int32 {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.PduSessionIDVal
}

func (c *SmContext) SmContextRef() string {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.SmContextRefVal
}

func (c *SmContext) SetSmContextRef(ref string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.SmContextRefVal = ref
}

func (c *SmContext) Snssai() models.Snssai {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.SnssaiVal
}

func (c *SmContext) SetSnssai(snssai models.Snssai) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.SnssaiVal = snssai
}

func (c *SmContext) Dnn() string {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.DnnVal
}

func (c *SmContext) SetDnn(dnn string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.DnnVal = dnn
}

func (c *SmContext) UserLocation() models.UserLocation {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.UserLocationVal
}

func (c *SmContext) SetUserLocation(userLocation models.UserLocation) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.UserLocationVal = userLocation
}
