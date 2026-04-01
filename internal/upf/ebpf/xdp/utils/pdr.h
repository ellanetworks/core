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

#define MAX_RULES_PER_FILTER    12  /* max rules per policy-direction entry */
#define MAX_POLICIES            12  /* max number of policies supported */
#define MAX_SDF_FILTERS         (2 * MAX_POLICIES)  /* one slot per policy-direction pair */
#define SDF_PROTO_ANY          255  /* wildcard protocol */
#define SDF_PORT_ANY             0  /* wildcard port (low == high == 0 means any) */

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

struct sdf_rule {
	__u32 remote_ip;    /* network-order IPv4 prefix base; 0 = wildcard */
	__u32 remote_mask;  /* prefix mask; 0 = wildcard */
	__u16 port_low;     /* dest port range low bound; 0 = wildcard */
	__u16 port_high;    /* dest port range high bound; 0 = wildcard */
	__u8  protocol;     /* IP protocol; SDF_PROTO_ANY (255) = wildcard */
	__u8  action;       /* 0 = allow, 1 = deny */
	__u8  pad[2];       /* explicit padding for 4-byte alignment */
};

struct sdf_filter_list {
	__u8             num_rules;              /* number of valid entries in rules[] */
	__u8             pad[3];
	struct sdf_rule  rules[MAX_RULES_PER_FILTER];
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

enum gate_status {
	GATE_STATUS_OPEN = 0,
	GATE_STATUS_CLOSED = 1,
	GATE_STATUS_RESERVED1 = 2,
	GATE_STATUS_RESERVED2 = 3,
};

struct qer_info {
	__u8 ul_gate_status;
	__u8 dl_gate_status;
	__u8 qfi;
	__u64 ul_maximum_bitrate;
	__u64 dl_maximum_bitrate;
	volatile __u64 ul_start;
	volatile __u64 dl_start;
};

struct pdr_info {
	__u64 local_seid;
	__u64 imsi;
	__u32 pdr_id;
	__u32 urr_id;
	__u8 outer_header_removal;
	__u8 pad[3];           /* explicit padding */
	struct far_info far;
	struct qer_info qer;
	__u32 filter_map_index; /* 0 = no SDF filtering for this PDR */
};
