// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/nas"
)

func TestBuildAuthorizedQosFlowDescriptions(t *testing.T) {
	authorizedQosFlow, err := nas.BuildAuthorizedQosFlowDescription(&models.QosData{
		QFI:    1,
		Var5qi: 5,
	})
	if err != nil {
		t.Fatalf("Error building Authorized QoS Flow Descriptions: %v", err)
	}

	expectedBytes := []byte{0x1, 0x20, 0x41, 0x1, 0x1, 0x5}
	if string(expectedBytes) != string(authorizedQosFlow.Content) {
		t.Errorf("Expected: %v, got: %v", expectedBytes, authorizedQosFlow.Content)
	}
}
