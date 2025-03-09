// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

type PFCPSessionContext struct {
	PDRs       map[uint16]*PDR
	NodeID     NodeID
	LocalSEID  uint64
	RemoteSEID uint64
}
