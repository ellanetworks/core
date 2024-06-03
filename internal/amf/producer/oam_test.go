// SPDX-FileCopyrightText: 2022 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package producer

import (
	"fmt"
	"testing"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/fsm"
	"github.com/stretchr/testify/assert"
	"github.com/yeastengine/canard/internal/amf/context"
	"github.com/yeastengine/canard/internal/amf/factory"
	"github.com/yeastengine/canard/internal/amf/gmm"
	"github.com/yeastengine/canard/internal/amf/util"
)

func init() {
	if err := factory.InitConfigFactory("../amfTest/amfcfg.yaml"); err != nil {
		fmt.Printf("Error in InitConfigFactory: %v\n", err)
	}

	self := context.AMF_Self()
	util.InitAmfContext(self)

	gmm.Mockinit()
}

func TestHandleOAMPurgeUEContextRequest_UEDeregistered(t *testing.T) {
	self := context.AMF_Self()
	var err error
	self.Drsm, err = util.MockDrsmInit()
	if err != nil {
		fmt.Printf("Error in MockDrsmInit: %v\n", err)
	}
	amfUe := self.NewAmfUe("imsi-208930100007497")

	HandleOAMPurgeUEContextRequest(amfUe.Supi, "", nil)

	if _, ok := self.AmfUeFindBySupi(amfUe.Supi); ok {
		t.Errorf("test failed")
	}

	assert.Equal(t, uint32(0), gmm.MockDeregisteredInitiatedCallCount)
	assert.Equal(t, uint32(0), gmm.MockRegisteredCallCount)
}

func TestHandleOAMPurgeUEContextRequest_UERegistered(t *testing.T) {
	self := context.AMF_Self()
	amfUe := self.NewAmfUe("imsi-208930100007497")
	amfUe.State[models.AccessType__3_GPP_ACCESS] = fsm.NewState(context.Registered)

	HandleOAMPurgeUEContextRequest(amfUe.Supi, "", nil)

	if _, ok := self.AmfUeFindBySupi(amfUe.Supi); ok {
		t.Errorf("test failed")
	}

	assert.Equal(t, uint32(2), gmm.MockRegisteredCallCount)
	assert.Equal(t, uint32(1), gmm.MockDeregisteredInitiatedCallCount)
}
