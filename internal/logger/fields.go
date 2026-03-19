// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package logger

import "go.uber.org/zap"

// UE Identity
func SUPI(val string) zap.Field { return zap.String("supi", val) }
func IMSI(val string) zap.Field { return zap.String("imsi", val) }
func GUTI(val string) zap.Field { return zap.String("guti", val) }
func PEI(val string) zap.Field  { return zap.String("pei", val) }
func TMSI(val string) zap.Field { return zap.String("tmsi", val) }

// Session & NGAP
func AmfUeNgapID(val int64) zap.Field  { return zap.Int64("amf_ue_ngap_id", val) }
func RanUeNgapID(val int64) zap.Field  { return zap.Int64("ran_ue_ngap_id", val) }
func PDUSessionID(val uint8) zap.Field { return zap.Uint8("pdu_session_id", val) }
func DNN(val string) zap.Field         { return zap.String("dnn", val) }
func SST(val uint8) zap.Field          { return zap.Uint8("sst", val) }
func SD(val string) zap.Field          { return zap.String("sd", val) }
func Cause(val string) zap.Field       { return zap.String("cause", val) }
func MessageType(val string) zap.Field { return zap.String("message_type", val) }

// PFCP Rules
func SEID(val uint64) zap.Field  { return zap.Uint64("seid", val) }
func PDRID(val uint32) zap.Field { return zap.Uint32("pdr_id", val) }
func FARID(val uint32) zap.Field { return zap.Uint32("far_id", val) }
func QERID(val uint32) zap.Field { return zap.Uint32("qer_id", val) }
func URRID(val uint32) zap.Field { return zap.Uint32("urr_id", val) }
func QFI(val uint8) zap.Field    { return zap.Uint8("qfi", val) }

// Network & Transport
func RanAddr(val string) zap.Field         { return zap.String("ran_addr", val) }
func SourceIP(val string) zap.Field        { return zap.String("source_ip", val) }
func DestinationIP(val string) zap.Field   { return zap.String("destination_ip", val) }
func SourcePort(val uint16) zap.Field      { return zap.Uint16("source_port", val) }
func DestinationPort(val uint16) zap.Field { return zap.Uint16("destination_port", val) }
func Protocol(val uint8) zap.Field         { return zap.Uint8("protocol", val) }
func ProtocolName(val string) zap.Field    { return zap.String("protocol", val) }
func IPAddress(val string) zap.Field       { return zap.String("ip_address", val) }
func TEID(val uint32) zap.Field            { return zap.Uint32("teid", val) }
func Direction(val string) zap.Field       { return zap.String("direction", val) }
func N3Address(val string) zap.Field       { return zap.String("n3_address", val) }

// Metrics & Volume
func Packets(val uint64) zap.Field        { return zap.Uint64("packets", val) }
func Bytes(val uint64) zap.Field          { return zap.Uint64("bytes", val) }
func UplinkVolume(val uint64) zap.Field   { return zap.Uint64("uplink_volume", val) }
func DownlinkVolume(val uint64) zap.Field { return zap.Uint64("downlink_volume", val) }
