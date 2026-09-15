// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// UE Identity
func SUPI(val string) zap.Field { return identity("supi", val) }
func GUTI(val string) zap.Field { return identity("guti", val) }
func PEI(val string) zap.Field  { return identity("pei", val) }
func TMSI(val string) zap.Field { return identity("tmsi", val) }

func SUPIFromIMSI(imsi string) zap.Field {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return zap.Skip()
	}

	return SUPI(supi.String())
}

func identity(key, val string) zap.Field {
	if val == "" {
		return zap.Skip()
	}

	return zap.String(key, val)
}

// Session, NGAP & S1AP
func AmfUeNgapID(val models.AmfUeNgapID) zap.Field { return zap.Int64("amf_ue_ngap_id", int64(val)) }

func RanUeNgapID(val models.RanUeNgapID) zap.Field { return zap.Int64("ran_ue_ngap_id", int64(val)) }
func MMEUeS1apID(val uint32) zap.Field             { return zap.Uint32("mme_ue_s1ap_id", val) }
func ENBUeS1apID(val uint32) zap.Field             { return zap.Uint32("enb_ue_s1ap_id", val) }
func PDUSessionID(val uint8) zap.Field             { return zap.Uint8("pdu_session_id", val) }
func ERABID(val uint8) zap.Field                   { return zap.Uint8("e_rab_id", val) }
func SMContextRef(val string) zap.Field            { return zap.String("sm_context_ref", val) }
func ESMCause(val string) zap.Field                { return zap.String("esm_cause", val) }
func FiveGSMCause(val uint8) zap.Field             { return zap.Uint8("5gsm_cause", val) }
func DNN(val string) zap.Field                     { return zap.String("dnn", val) }
func SST(val uint8) zap.Field                      { return zap.Uint8("sst", val) }
func SD(val string) zap.Field                      { return zap.String("sd", val) }
func Cause(val string) zap.Field                   { return zap.String("cause", val) }
func MessageType(val string) zap.Field             { return zap.String("message_type", val) }

// PFCP Rules
func SEID(val uint64) zap.Field  { return zap.Uint64("seid", val) }
func PDRID(val uint32) zap.Field { return zap.Uint32("pdr_id", val) }
func FARID(val uint32) zap.Field { return zap.Uint32("far_id", val) }
func QERID(val uint32) zap.Field { return zap.Uint32("qer_id", val) }
func URRID(val uint32) zap.Field { return zap.Uint32("urr_id", val) }
func QFI(val uint8) zap.Field    { return zap.Uint8("qfi", val) }

// Network & Transport
func RanAddr(val string) zap.Field      { return zap.String("ran_addr", val) }
func RadioName(val string) zap.Field    { return identity("radio_name", val) }
func RadioID(val string) zap.Field      { return identity("radio_id", val) }
func ProtocolName(val string) zap.Field { return zap.String("protocol", val) }
func IPAddress(val string) zap.Field    { return zap.String("ip_address", val) }
func IPv6Prefix(val string) zap.Field   { return zap.String("ipv6_prefix", val) }
func IPv6IID(val string) zap.Field      { return zap.String("ipv6_iid", val) }
func TEID(val uint32) zap.Field         { return zap.Uint32("teid", val) }
func Direction(val string) zap.Field    { return zap.String("direction", val) }
func N3Address(val string) zap.Field    { return zap.String("n3_address", val) }

// Metrics & Volume
func Packets(val uint64) zap.Field        { return zap.Uint64("packets", val) }
func Bytes(val uint64) zap.Field          { return zap.Uint64("bytes", val) }
func UplinkVolume(val uint64) zap.Field   { return zap.Uint64("uplink_volume", val) }
func DownlinkVolume(val uint64) zap.Field { return zap.Uint64("downlink_volume", val) }
