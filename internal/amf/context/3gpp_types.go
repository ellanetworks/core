// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"time"

	"github.com/ellanetworks/core/internal/models"
)

const (
	MaxNumOfTAI             int   = 16
	MaxNumOfBroadcastPLMNs  int   = 12
	MaxNumOfSlice           int   = 1024
	MaxValueOfAmfUeNgapId   int64 = 1099511627775
	MaxNumOfServedGuamiList int   = 256
	MaxNumOfPDUSessions     int   = 256
	MaxNumOfAOI             int   = 64
)

// timers defined in TS 24.501 table 10.2.2
const (
	TimeT3513 time.Duration = 6 * time.Second
)

type LADN struct {
	Dnn      string
	TaiLists []models.Tai
}

type CauseAll struct {
	Cause        *models.Cause
	NgapCause    *models.NgApCause
	Var5GmmCause *int32
}
