// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

// Algorithm type distinguishers P0 for NAS key derivation with FC=0x69
// (TS 33.501 Annex A.8, table A.8-1).
const (
	nnasEncAlgDistinguisher uint8 = 0x01
	nnasIntAlgDistinguisher uint8 = 0x02
)

// anKeyAccessType3GPP is the access-type distinguisher used in KgNB/KN3IWF
// derivation (TS 33.501 Annex A.9).
const anKeyAccessType3GPP uint8 = 0x01
