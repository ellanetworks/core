// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"net"

	"github.com/ellanetworks/core/internal/models"
)

// SnssaiSmfInfo records the SMF S-NSSAI related information
type SnssaiSmfInfo struct {
	DnnInfos map[string]*SnssaiSmfDnnInfo
	PlmnID   models.PlmnID
	Snssai   SNssai
}

// SnssaiSmfDnnInfo records the SMF per S-NSSAI DNN information
type SnssaiSmfDnnInfo struct {
	DNS DNS
	MTU uint16
}

type DNS struct {
	IPv4Addr net.IP
	IPv6Addr net.IP
}
