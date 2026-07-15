// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import AccessChip from "@/components/AccessChip";

/**
 * Node types reported by the core: the AMF sends gNB, ng-eNB and N3IWF
 * (internal/amf/amf_radio.go), while the MME path reports eNB
 * (internal/api/server/api_radios.go).
 */
const RAN_NODE_TYPE_LABELS: Record<string, string> = {
  gNB: "gNB (5G)",
  "ng-eNB": "ng-eNB (4G)",
  eNB: "eNB (4G)",
  N3IWF: "N3IWF",
};

const RanNodeTypeChip: React.FC<{ type: string }> = ({ type }) => (
  <AccessChip label={RAN_NODE_TYPE_LABELS[type] ?? type} />
);

export default RanNodeTypeChip;
