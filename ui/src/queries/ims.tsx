// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export interface IMSApplicationServer {
  name: string;
  server_name: string;
  public_service_identities: string[];
}

export type IMSSessionCase =
  | "ORIGINATING_REGISTERED"
  | "TERMINATING_REGISTERED"
  | "TERMINATING_UNREGISTERED"
  | "ORIGINATING_UNREGISTERED"
  | "ORIGINATING_CDIV";

export interface IMSServicePointTrigger {
  condition_negated?: boolean;
  group: number[];
  request_uri?: string;
  method?: string;
  sip_header?: { header: string; content?: string };
  session_case?: IMSSessionCase;
  session_description?: { line: string; content?: string };
  registration_type?: string[];
}

export interface IMSInitialFilterCriterion {
  priority: number;
  application_server_name: string;
  trigger_point?: {
    condition_type_cnf: boolean;
    spt: IMSServicePointTrigger[];
  };
  profile_part?: "REGISTERED" | "UNREGISTERED";
  default_handling?: "SESSION_CONTINUED" | "SESSION_TERMINATED";
  service_info?: string;
  include_register_request?: boolean;
  include_register_response?: boolean;
}

export type IMSInitialFilterCriterionInput = Omit<
  IMSInitialFilterCriterion,
  "priority"
>;

const AS_PATH = "/api/v1/operator/ims/application-servers";

export const listIMSApplicationServers = async (
  authToken: string,
): Promise<IMSApplicationServer[]> => {
  const res = await apiFetch<{ items: IMSApplicationServer[] }>(AS_PATH, {
    authToken,
  });
  return res.items ?? [];
};

export const createIMSApplicationServer = async (
  authToken: string,
  as: IMSApplicationServer,
): Promise<void> => {
  await apiFetchVoid(AS_PATH, { method: "POST", authToken, body: as });
};

export const updateIMSApplicationServer = async (
  authToken: string,
  name: string,
  as: Omit<IMSApplicationServer, "name">,
): Promise<void> => {
  await apiFetchVoid(`${AS_PATH}/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: as,
  });
};

export const deleteIMSApplicationServer = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`${AS_PATH}/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};

const profileFilterCriteriaPath = (profileName: string) =>
  `/api/v1/profiles/${encodeURIComponent(profileName)}/ims-filter-criteria`;

export const getProfileIMSFilterCriteria = async (
  authToken: string,
  profileName: string,
): Promise<IMSInitialFilterCriterion[]> => {
  const res = await apiFetch<{
    initial_filter_criteria: IMSInitialFilterCriterion[];
  }>(profileFilterCriteriaPath(profileName), { authToken });
  return res.initial_filter_criteria ?? [];
};

export const setProfileIMSFilterCriteria = async (
  authToken: string,
  profileName: string,
  criteria: IMSInitialFilterCriterionInput[],
): Promise<void> => {
  await apiFetchVoid(profileFilterCriteriaPath(profileName), {
    method: "PUT",
    authToken,
    body: { initial_filter_criteria: criteria },
  });
};

export const isSIPURI = (value: string): boolean =>
  /^(sips?|tel):\S+$/i.test(value.trim());

export const describeTrigger = (spt: IMSServicePointTrigger): string => {
  const not = spt.condition_negated ? "not " : "";
  if (spt.method !== undefined) {
    const types = spt.registration_type?.length
      ? ` (${spt.registration_type.join(", ")})`
      : "";
    return `${not}method ${spt.method}${types}`;
  }
  if (spt.request_uri !== undefined)
    return `${not}request URI ${spt.request_uri}`;
  if (spt.session_case !== undefined)
    return `${not}session case ${spt.session_case}`;
  if (spt.sip_header !== undefined) {
    const content = spt.sip_header.content
      ? ` = ${spt.sip_header.content}`
      : "";
    return `${not}header ${spt.sip_header.header}${content}`;
  }
  if (spt.session_description !== undefined) {
    const content = spt.session_description.content
      ? ` = ${spt.session_description.content}`
      : "";
    return `${not}SDP ${spt.session_description.line}${content}`;
  }
  return `${not}any`;
};

export const describeTriggerPoint = (
  ifc: IMSInitialFilterCriterion,
): string => {
  const spts = ifc.trigger_point?.spt ?? [];
  if (spts.length === 0) return "Always";
  const groups = new Map<number, string[]>();
  for (const spt of spts) {
    for (const g of spt.group.length ? spt.group : [0]) {
      groups.set(g, [...(groups.get(g) ?? []), describeTrigger(spt)]);
    }
  }
  const cnf = ifc.trigger_point?.condition_type_cnf ?? false;
  const inner = cnf ? " or " : " and ";
  const outer = cnf ? " and " : " or ";
  const parts = [...groups.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, items]) =>
      items.length > 1 && groups.size > 1
        ? `(${items.join(inner)})`
        : items.join(inner),
    );
  return parts.join(outer);
};
