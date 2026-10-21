// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export interface IMSApplicationServer {
  name: string;
  server_name: string;
  public_service_identities: string[];
}

export type IMSServiceKind = "mcptt" | "custom";

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
  trigger_point?: {
    condition_type_cnf: boolean;
    spt: IMSServicePointTrigger[];
  };
  default_handling: "SESSION_CONTINUED" | "SESSION_TERMINATED";
  include_register_request?: boolean;
  include_register_response?: boolean;
}

export interface IMSService {
  name: string;
  kind: IMSServiceKind;
  application_server_name: string;
  initial_filter_criteria: IMSInitialFilterCriterion[];
}

const AS_PATH = "/api/v1/operator/ims/application-servers";
const SERVICES_PATH = "/api/v1/operator/ims/services";

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

export const listIMSServices = async (
  authToken: string,
): Promise<IMSService[]> => {
  const res = await apiFetch<{ items: IMSService[] }>(SERVICES_PATH, {
    authToken,
  });
  return res.items ?? [];
};

export const createIMSService = async (
  authToken: string,
  service: {
    name: string;
    kind: IMSServiceKind;
    application_server_name: string;
  },
): Promise<void> => {
  await apiFetchVoid(SERVICES_PATH, {
    method: "POST",
    authToken,
    body: service,
  });
};

export const updateIMSService = async (
  authToken: string,
  name: string,
  service: { kind: IMSServiceKind; application_server_name: string },
): Promise<void> => {
  await apiFetchVoid(`${SERVICES_PATH}/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: service,
  });
};

export const deleteIMSService = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`${SERVICES_PATH}/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};

export const getProfileIMSServices = async (
  authToken: string,
  profileName: string,
): Promise<string[]> => {
  const res = await apiFetch<{ services: string[] }>(
    `/api/v1/profiles/${encodeURIComponent(profileName)}/ims-services`,
    { authToken },
  );
  return res.services ?? [];
};

export const updateProfileIMSServices = async (
  authToken: string,
  profileName: string,
  services: string[],
): Promise<void> => {
  await apiFetchVoid(
    `/api/v1/profiles/${encodeURIComponent(profileName)}/ims-services`,
    { method: "PUT", authToken, body: { services } },
  );
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
