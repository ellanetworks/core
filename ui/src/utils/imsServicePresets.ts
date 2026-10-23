// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type {
  IMSApplicationServer,
  IMSInitialFilterCriterionInput,
} from "@/queries/ims";

export type IMSPresetFilterCriterion = Omit<
  IMSInitialFilterCriterionInput,
  "application_server_name"
>;

export interface IMSServicePreset {
  name: string;
  label: string;
  description: string;
  initial_filter_criteria: IMSPresetFilterCriterion[];
  suggestServer: (
    servers: IMSApplicationServer[],
    homeDomain?: string,
  ) => IMSApplicationServer | undefined;
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
    suggestServer: (servers, homeDomain) =>
      servers.find(
        (s) => !!homeDomain && s.server_name === `sip:mcptt.${homeDomain}`,
      ) ?? servers.find((s) => s.name === "mcptt"),
    initial_filter_criteria: [
      {
        trigger: {
          conditions_match: "all",
          groups: [{ conditions: [{ method: "REGISTER" }] }],
        },
        default_handling: "SESSION_CONTINUED",
        include_register_request: true,
        include_register_response: true,
      },
      {
        trigger: {
          conditions_match: "all",
          groups: [
            {
              conditions: [
                { session_case: "ORIGINATING_REGISTERED" },
                {
                  sip_header: {
                    header: "^P-Asserted-Service$",
                    content: "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$",
                  },
                },
              ],
            },
          ],
        },
        default_handling: "SESSION_TERMINATED",
      },
    ],
  },
];
