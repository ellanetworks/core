// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// AmfUeNgapID is the AMF-UE-NGAP-ID, INTEGER (0..2^40-1) (TS 38.413 §9.3.3.1). The
// 40-bit range forces a 64-bit carrier, unlike the S1AP MME-UE-S1AP-ID (32-bit).
type AmfUeNgapID int64

// RanUeNgapID is the RAN-UE-NGAP-ID, INTEGER (0..2^32-1) (TS 38.413 §9.3.3.2), carried
// as int64 to match the value free5gc's ngap codec yields.
type RanUeNgapID int64

// RanUeNgapIDUnspecified is the reserved value marking a RAN-UE-NGAP-ID as not yet
// assigned, e.g. a handover target before the target RAN allocates one.
const RanUeNgapIDUnspecified RanUeNgapID = 0xffffffff
