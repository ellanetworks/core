/**
 * Copyright 2023 Edgecom LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/ipv6.h>

#define MAX_UES 1000
#define MAX_PDU_SESSIONS (2 * MAX_UES)
#define PDR_MAP_UPLINK_SIZE MAX_PDU_SESSIONS
#define PDR_MAP_DOWNLINK_IPV4_SIZE MAX_PDU_SESSIONS
#define PDR_MAP_DOWNLINK_IPV6_SIZE MAX_PDU_SESSIONS
#define FAR_MAP_SIZE MAX_PDU_SESSIONS * 2

enum outer_header_removal_values {
	OHR_GTP_U_UDP_IPv4 = 0,
	OHR_GTP_U_UDP_IPv6 = 1,
	OHR_UDP_IPv4 = 2,
	OHR_UDP_IPv6 = 3,
	OHR_IPv4 = 4,
	OHR_IPv6 = 5,
	OHR_GTP_U_UDP_IP = 6,
	OHR_VLAN_S_TAG = 7,
	OHR_S_TAG_C_TAG = 8,
};

struct pdr_info {
	__u64 local_seid;
	__u64 imsi;
	__u32 pdr_id;
	__u32 far_id;
	__u32 qer_id;
	__u32 urr_id;
	__u8 outer_header_removal;
};

enum far_action_mask {
	FAR_DROP = 0x01,
	FAR_FORW = 0x02,
	FAR_BUFF = 0x04,
	FAR_NOCP = 0x08,
	FAR_DUPL = 0x10,
	FAR_IPMA = 0x20,
	FAR_IPMD = 0x40,
	FAR_DFRT = 0x80,
};

enum outer_header_creation_values {
	OHC_GTP_U_UDP_IPv4 = 0x01,
	OHC_GTP_U_UDP_IPv6 = 0x02,
	OHC_UDP_IPv4 = 0x04,
	OHC_UDP_IPv6 = 0x08,
};

struct far_info {
	__u8 action;
	__u8 outer_header_creation;
	__u32 teid;
	__u32 remoteip;
	__u32 localip;
	/* first octet DSCP value in the Type-of-Service, second octet shall contain the ToS/Traffic Class mask field, which shall be set to "0xFC". */
	__u16 transport_level_marking;
};

/* FAR ID -> FAR */
struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct far_info);
	__uint(max_entries, FAR_MAP_SIZE);
} far_map SEC(".maps");
