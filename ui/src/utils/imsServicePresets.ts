// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type {
  IMSApplicationServer,
  IMSInitialFilterCriterionInput,
} from "@/queries/ims";

export type IMSPresetFilterCriterion = Omit<
  IMSInitialFilterCriterionInput,
  "applicationServerName"
>;

export interface IMSServicePreset {
  name: string;
  label: string;
  description: string;
  initialFilterCriteria: IMSPresetFilterCriterion[];
  suggestServer: (
    servers: IMSApplicationServer[],
    homeDomain?: string,
  ) => IMSApplicationServer | undefined;
}

export const applyPreset = (
  preset: IMSServicePreset,
  applicationServerName: string,
): IMSInitialFilterCriterionInput[] =>
  preset.initialFilterCriteria.map((ifc) => ({
    applicationServerName: applicationServerName,
    ...ifc,
  }));

export const imsServicePresets: IMSServicePreset[] = [
  {
    name: "mcptt",
    label: "MCPTT",
    description:
      "For the MCPTT registration set. MCPTT server: third-party registration and originating MCPTT sessions",
    suggestServer: (servers, homeDomain) =>
      servers.find(
        (s) => !!homeDomain && s.serverName === `sip:mcptt.${homeDomain}`,
      ) ?? servers.find((s) => s.name === "mcptt"),
    initialFilterCriteria: [
      {
        trigger: {
          conditionsMatch: "all",
          groups: [{ conditions: [{ method: "REGISTER" }] }],
        },
        defaultHandling: "SESSION_CONTINUED",
        includeRegisterRequest: true,
        includeRegisterResponse: true,
      },
      {
        trigger: {
          conditionsMatch: "all",
          groups: [
            {
              conditions: [
                { sessionCase: "ORIGINATING_REGISTERED" },
                {
                  sipHeader: {
                    header: "^P-Asserted-Service$",
                    content: "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$",
                  },
                },
              ],
            },
          ],
        },
        defaultHandling: "SESSION_TERMINATED",
      },
    ],
  },
  {
    name: "sms-over-ip",
    label: "SMS over IP",
    description:
      "For the MMTel registration set. IP-SM-GW: third-party registration and originating short messages",
    suggestServer: (servers, homeDomain) =>
      servers.find(
        (s) => !!homeDomain && s.serverName === `sip:ipsmgw.${homeDomain}`,
      ) ?? servers.find((s) => s.name === "ip-sm-gw"),
    initialFilterCriteria: [
      {
        trigger: {
          conditionsMatch: "all",
          groups: [{ conditions: [{ method: "REGISTER" }] }],
        },
        defaultHandling: "SESSION_CONTINUED",
        includeRegisterRequest: true,
        includeRegisterResponse: true,
      },
      {
        trigger: {
          conditionsMatch: "all",
          groups: [
            {
              conditions: [
                { method: "MESSAGE" },
                { sessionCase: "ORIGINATING_REGISTERED" },
                {
                  sipHeader: {
                    header: "^Content-Type$",
                    content: "^application/vnd\\.3gpp\\.sms$",
                  },
                },
              ],
            },
          ],
        },
        defaultHandling: "SESSION_CONTINUED",
      },
    ],
  },
];
