// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package attrs

import "go.opentelemetry.io/otel/attribute"

func SUPI(val string) attribute.KeyValue { return attribute.String("ue.supi", val) }

func IMSI(val string) attribute.KeyValue { return attribute.String("ue.imsi", val) }

func SUCI(val string) attribute.KeyValue { return attribute.String("ue.suci", val) }

func PDUSessionID(val uint8) attribute.KeyValue {
	return attribute.Int("pdu_session.id", int(val))
}

func DNN(val string) attribute.KeyValue { return attribute.String("data_network.name", val) }

func DataNetworkID(val string) attribute.KeyValue {
	return attribute.String("data_network.id", val)
}

func SST(val int32) attribute.KeyValue { return attribute.Int("slice.sst", int(val)) }

func SD(val string) attribute.KeyValue { return attribute.String("slice.sd", val) }

func SliceID(val string) attribute.KeyValue { return attribute.String("slice.id", val) }

func ProfileID(val string) attribute.KeyValue { return attribute.String("profile.id", val) }

func SEID(val uint64) attribute.KeyValue { return attribute.Int64("session.seid", int64(val)) }

func SessionOperation(val string) attribute.KeyValue {
	return attribute.String("session.operation", val)
}

func SMContextRef(val string) attribute.KeyValue {
	return attribute.String("smf.sm_context_ref", val)
}

func NodeID(val int) attribute.KeyValue { return attribute.Int("cluster.node_id", val) }

func LeaseIPv4(val string) attribute.KeyValue { return attribute.String("ip_lease.ipv4", val) }

func LeaseIPv6(val string) attribute.KeyValue { return attribute.String("ip_lease.ipv6", val) }
