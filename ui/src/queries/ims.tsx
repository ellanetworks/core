// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export interface IMSApplicationServer {
  name: string;
  serverName: string;
  publicServiceIdentities: string[];
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
  requestURI?: string;
  method?: string;
  registrationTypes?: IMSRegistrationType[];
  sipHeader?: { header: string; content?: string };
  sessionCase?: IMSSessionCase;
  sessionDescription?: { line: string; content?: string };
}

export interface IMSConditionGroup {
  conditions: IMSCondition[];
}

export type IMSConditionsMatch = "all" | "any";

export interface IMSTrigger {
  conditionsMatch: IMSConditionsMatch;
  groups: IMSConditionGroup[];
}

export interface IMSInitialFilterCriterion {
  priority: number;
  applicationServerName: string;
  trigger?: IMSTrigger;
  profilePart?: "REGISTERED" | "UNREGISTERED";
  defaultHandling?: "SESSION_CONTINUED" | "SESSION_TERMINATED";
  serviceInfo?: string;
  includeRegisterRequest?: boolean;
  includeRegisterResponse?: boolean;
}

export type IMSInitialFilterCriterionInput = Omit<
  IMSInitialFilterCriterion,
  "priority"
>;

const AS_PATH = "/api/v1/integrations/ims/application-servers";

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
  initialFilterCriteria: IMSInitialFilterCriterion[];
}

export type IMSRegistrationSet = "mmtel" | "mcptt";

export const imsRegistrationSets: IMSRegistrationSet[] = ["mmtel", "mcptt"];

export const imsRegistrationSetLabels: Record<IMSRegistrationSet, string> = {
  mmtel: "MMTel (voice, SMS)",
  mcptt: "MCPTT",
};

export const imsRegistrationSetIdentities: Record<IMSRegistrationSet, string> =
  {
    mmtel: "sip:+MSISDN, tel:+MSISDN, barred IMSI",
    mcptt: "sip:mcptt-MSISDN",
  };

export const imsUnassignedLabels: Record<IMSRegistrationSet, string> = {
  mmtel: "None (basic calls only)",
  mcptt: "Off",
};

export type ProfileIMSServiceProfiles = Partial<
  Record<IMSRegistrationSet, string | null>
>;

const SP_PATH = "/api/v1/integrations/ims/service-profiles";

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
    body: { name, initialFilterCriteria: criteria },
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
    body: { initialFilterCriteria: criteria },
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
    const types = c.registrationTypes?.length
      ? ` (${c.registrationTypes.map((t) => registrationTypeLabels[t]).join(", ")})`
      : "";
    return `method ${c.negated ? "is not" : "is"} ${c.method}${types}`;
  }
  if (c.requestURI !== undefined)
    return `Request-URI ${c.negated ? "does not match" : "matches"} ${c.requestURI}`;
  if (c.sessionCase !== undefined)
    return `${not}${sessionCaseLabels[c.sessionCase] ?? c.sessionCase}`;
  if (c.sipHeader !== undefined) {
    const { header, content } = c.sipHeader;
    if (!content) return `header ${header} ${c.negated ? "absent" : "present"}`;
    return `header ${header} ${c.negated ? "does not match" : "matches"} ${content}`;
  }
  if (c.sessionDescription !== undefined) {
    const { line, content } = c.sessionDescription;
    if (!content) return `SDP line ${line} ${c.negated ? "absent" : "present"}`;
    return `SDP line ${line} ${c.negated ? "does not match" : "matches"} ${content}`;
  }
  return `${not}unknown condition`;
};

export const describeTrigger = (trigger?: IMSTrigger): string => {
  if (!trigger || trigger.groups.length === 0) return "Always";
  const all = trigger.conditionsMatch === "all";
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
