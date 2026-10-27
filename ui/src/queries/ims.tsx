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

export type IMSRegistrationType =
  "INITIAL_REGISTRATION" | "RE-REGISTRATION" | "DE-REGISTRATION";

export interface IMSCondition {
  negated?: boolean;
  request_uri?: string;
  method?: string;
  registration_types?: IMSRegistrationType[];
  sip_header?: { header: string; content?: string };
  session_case?: IMSSessionCase;
  session_description?: { line: string; content?: string };
}

export interface IMSConditionGroup {
  conditions: IMSCondition[];
}

export type IMSConditionsMatch = "all" | "any";

export interface IMSTrigger {
  conditions_match: IMSConditionsMatch;
  groups: IMSConditionGroup[];
}

export interface IMSInitialFilterCriterion {
  priority: number;
  application_server_name: string;
  trigger?: IMSTrigger;
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

export interface IMSServiceProfile {
  name: string;
  initial_filter_criteria: IMSInitialFilterCriterion[];
}

export type IMSRegistrationSet = "mmtel" | "mcptt";

export const imsRegistrationSets: IMSRegistrationSet[] = ["mmtel", "mcptt"];

export const imsRegistrationSetLabels: Record<IMSRegistrationSet, string> = {
  mmtel: "MMTel",
  mcptt: "MCPTT",
};

export type ProfileIMSServiceProfiles = Partial<
  Record<IMSRegistrationSet, string | null>
>;

const SP_PATH = "/api/v1/operator/ims/service-profiles";

export const listIMSServiceProfiles = async (
  authToken: string,
): Promise<IMSServiceProfile[]> => {
  const res = await apiFetch<{ items: IMSServiceProfile[] }>(SP_PATH, {
    authToken,
  });
  return res.items ?? [];
};

export const createIMSServiceProfile = async (
  authToken: string,
  name: string,
  criteria: IMSInitialFilterCriterionInput[],
): Promise<void> => {
  await apiFetchVoid(SP_PATH, {
    method: "POST",
    authToken,
    body: { name, initial_filter_criteria: criteria },
  });
};

export const updateIMSServiceProfile = async (
  authToken: string,
  name: string,
  criteria: IMSInitialFilterCriterionInput[],
): Promise<void> => {
  await apiFetchVoid(`${SP_PATH}/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: { initial_filter_criteria: criteria },
  });
};

export const deleteIMSServiceProfile = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`${SP_PATH}/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};

const profileServiceProfilesPath = (profileName: string) =>
  `/api/v1/profiles/${encodeURIComponent(profileName)}/ims-service-profiles`;

export const getProfileIMSServiceProfiles = async (
  authToken: string,
  profileName: string,
): Promise<ProfileIMSServiceProfiles> =>
  apiFetch<ProfileIMSServiceProfiles>(profileServiceProfilesPath(profileName), {
    authToken,
  });

export const setProfileIMSServiceProfiles = async (
  authToken: string,
  profileName: string,
  assigned: ProfileIMSServiceProfiles,
): Promise<void> => {
  await apiFetchVoid(profileServiceProfilesPath(profileName), {
    method: "PUT",
    authToken,
    body: assigned,
  });
};

export const isSIPURI = (value: string): boolean =>
  /^(sips?|tel):\S+$/i.test(value.trim());

export const sessionCaseLabels: Record<IMSSessionCase, string> = {
  ORIGINATING_REGISTERED: "Originating (registered)",
  ORIGINATING_UNREGISTERED: "Originating (unregistered)",
  TERMINATING_REGISTERED: "Terminating (registered)",
  TERMINATING_UNREGISTERED: "Terminating (unregistered)",
  ORIGINATING_CDIV: "Originating after call diversion",
};

export const registrationTypeLabels: Record<IMSRegistrationType, string> = {
  INITIAL_REGISTRATION: "initial",
  "RE-REGISTRATION": "re-registration",
  "DE-REGISTRATION": "de-registration",
};

export const describeCondition = (c: IMSCondition): string => {
  const not = c.negated ? "not " : "";
  if (c.method !== undefined) {
    const types = c.registration_types?.length
      ? ` (${c.registration_types.map((t) => registrationTypeLabels[t]).join(", ")})`
      : "";
    return `method ${c.negated ? "is not" : "is"} ${c.method}${types}`;
  }
  if (c.request_uri !== undefined)
    return `Request-URI ${c.negated ? "does not match" : "matches"} ${c.request_uri}`;
  if (c.session_case !== undefined)
    return `${not}${sessionCaseLabels[c.session_case] ?? c.session_case}`;
  if (c.sip_header !== undefined) {
    const { header, content } = c.sip_header;
    if (!content) return `header ${header} ${c.negated ? "absent" : "present"}`;
    return `header ${header} ${c.negated ? "does not match" : "matches"} ${content}`;
  }
  if (c.session_description !== undefined) {
    const { line, content } = c.session_description;
    if (!content) return `SDP line ${line} ${c.negated ? "absent" : "present"}`;
    return `SDP line ${line} ${c.negated ? "does not match" : "matches"} ${content}`;
  }
  return `${not}unknown condition`;
};

export const describeTrigger = (trigger?: IMSTrigger): string => {
  if (!trigger || trigger.groups.length === 0) return "Always";
  const all = trigger.conditions_match === "all";
  const inner = all ? " and " : " or ";
  const outer = all ? " or " : " and ";
  const multipleGroups = trigger.groups.length > 1;
  return trigger.groups
    .map(({ conditions }) => {
      const text = conditions.map(describeCondition).join(inner);
      return multipleGroups && conditions.length > 1 ? `(${text})` : text;
    })
    .join(outer);
};
