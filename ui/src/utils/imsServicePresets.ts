// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { IMSInitialFilterCriterionInput } from "@/queries/ims";

export type IMSPresetFilterCriterion = Omit<
  IMSInitialFilterCriterionInput,
  "application_server_name"
>;

export interface IMSServicePreset {
  name: string;
  label: string;
  description: string;
  initial_filter_criteria: IMSPresetFilterCriterion[];
}

export const applyPreset = (
  preset: IMSServicePreset,
  applicationServerName: string,
): IMSInitialFilterCriterionInput[] =>
  preset.initial_filter_criteria.map((ifc) => ({
    application_server_name: applicationServerName,
    ...ifc,
  }));

export const imsServicePresets: IMSServicePreset[] = [
  {
    name: "mcptt",
    label: "MCPTT",
    description:
      "MCPTT server: third-party registration and originating MCPTT sessions",
    initial_filter_criteria: [
      {
        trigger_point: {
          condition_type_cnf: false,
          spt: [{ group: [0], method: "REGISTER" }],
        },
        default_handling: "SESSION_CONTINUED",
        include_register_request: true,
        include_register_response: true,
      },
      {
        trigger_point: {
          condition_type_cnf: true,
          spt: [
            { group: [0], session_case: "ORIGINATING_REGISTERED" },
            {
              group: [1],
              sip_header: {
                header: "^P-Asserted-Service$",
                content: "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$",
              },
            },
          ],
        },
        default_handling: "SESSION_TERMINATED",
      },
    ],
  },
];
