// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type {
  IMSCondition,
  IMSConditionsMatch,
  IMSInitialFilterCriterionInput,
  IMSRegistrationType,
  IMSSessionCase,
} from "@/queries/ims";

export type ConditionType =
  "method" | "requestURI" | "sipHeader" | "sessionCase" | "sessionDescription";

export type ConditionOperator = "is" | "is_not" | "present" | "absent";

export interface ConditionValues {
  type: ConditionType;
  operator: ConditionOperator;
  method: string;
  registrationTypes: IMSRegistrationType[];
  requestURI: string;
  header: string;
  header_content: string;
  sessionCase: IMSSessionCase;
  line: string;
  line_content: string;
}

export interface CriterionValues {
  applicationServerName: string;
  defaultHandling: "SESSION_CONTINUED" | "SESSION_TERMINATED";
  always: boolean;
  conditionsMatch: IMSConditionsMatch;
  groups: { conditions: ConditionValues[] }[];
  profilePart: "BOTH" | "REGISTERED" | "UNREGISTERED";
  serviceInfo: string;
  includeRegisterRequest: boolean;
  includeRegisterResponse: boolean;
}

export const allRegistrationTypes: IMSRegistrationType[] = [
  "INITIAL_REGISTRATION",
  "RE-REGISTRATION",
  "DE-REGISTRATION",
];

export const sipMethods = [
  "INVITE",
  "REGISTER",
  "MESSAGE",
  "SUBSCRIBE",
  "NOTIFY",
  "PUBLISH",
  "OPTIONS",
  "REFER",
  "INFO",
  "UPDATE",
];

export const emptyCondition = (): ConditionValues => ({
  type: "method",
  operator: "is",
  method: "",
  registrationTypes: [...allRegistrationTypes],
  requestURI: "",
  header: "",
  header_content: "",
  sessionCase: "ORIGINATING_REGISTERED",
  line: "",
  line_content: "",
});

const conditionToValues = (c: IMSCondition): ConditionValues => {
  const values = emptyCondition();
  values.operator = c.negated ? "is_not" : "is";
  if (c.method !== undefined) {
    values.method = c.method;
    if (c.registrationTypes?.length)
      values.registrationTypes = [...c.registrationTypes];
  } else if (c.requestURI !== undefined) {
    values.type = "requestURI";
    values.requestURI = c.requestURI;
  } else if (c.sipHeader !== undefined) {
    values.type = "sipHeader";
    values.header = c.sipHeader.header;
    values.header_content = c.sipHeader.content ?? "";
    if (c.sipHeader.content === undefined)
      values.operator = c.negated ? "absent" : "present";
  } else if (c.sessionCase !== undefined) {
    values.type = "sessionCase";
    values.sessionCase = c.sessionCase;
  } else if (c.sessionDescription !== undefined) {
    values.type = "sessionDescription";
    values.line = c.sessionDescription.line;
    values.line_content = c.sessionDescription.content ?? "";
    if (c.sessionDescription.content === undefined)
      values.operator = c.negated ? "absent" : "present";
  }
  return values;
};

export const criterionToValues = (
  ifc: IMSInitialFilterCriterionInput | undefined,
  defaultServer: string,
): CriterionValues => ({
  applicationServerName: ifc?.applicationServerName ?? defaultServer,
  defaultHandling: ifc?.defaultHandling ?? "SESSION_CONTINUED",
  always: !!ifc && !ifc.trigger,
  conditionsMatch: ifc?.trigger?.conditionsMatch ?? "all",
  groups: ifc?.trigger?.groups.length
    ? ifc.trigger.groups.map((g) => ({
        conditions: g.conditions.map(conditionToValues),
      }))
    : [{ conditions: [emptyCondition()] }],
  profilePart: ifc?.profilePart ?? "BOTH",
  serviceInfo: ifc?.serviceInfo ?? "",
  includeRegisterRequest: ifc?.includeRegisterRequest ?? false,
  includeRegisterResponse: ifc?.includeRegisterResponse ?? false,
});

export const isRegisterMethod = (c: ConditionValues) =>
  c.type === "method" && c.method.trim() === "REGISTER";

const conditionFromValues = (v: ConditionValues): IMSCondition => {
  const negated = v.operator === "is_not" || v.operator === "absent";
  const withContent = v.operator === "is" || v.operator === "is_not";
  const out: IMSCondition = negated ? { negated } : {};
  switch (v.type) {
    case "method":
      out.method = v.method.trim();
      if (
        isRegisterMethod(v) &&
        v.registrationTypes.length < allRegistrationTypes.length
      )
        out.registrationTypes = allRegistrationTypes.filter((t) =>
          v.registrationTypes.includes(t),
        );
      break;
    case "requestURI":
      out.requestURI = v.requestURI;
      break;
    case "sipHeader":
      out.sipHeader = withContent
        ? { header: v.header, content: v.header_content }
        : { header: v.header };
      break;
    case "sessionCase":
      out.sessionCase = v.sessionCase;
      break;
    case "sessionDescription":
      out.sessionDescription = withContent
        ? { line: v.line, content: v.line_content }
        : { line: v.line };
      break;
  }
  return out;
};

export const criterionFromValues = (
  v: CriterionValues,
): IMSInitialFilterCriterionInput => ({
  applicationServerName: v.applicationServerName,
  ...(v.always
    ? {}
    : {
        trigger: {
          conditionsMatch: v.conditionsMatch,
          groups: v.groups.map((g) => ({
            conditions: g.conditions.map(conditionFromValues),
          })),
        },
      }),
  defaultHandling: v.defaultHandling,
  ...(v.profilePart !== "BOTH" ? { profilePart: v.profilePart } : {}),
  ...(v.serviceInfo ? { serviceInfo: v.serviceInfo } : {}),
  includeRegisterRequest: v.includeRegisterRequest,
  includeRegisterResponse: v.includeRegisterResponse,
});

const canonical = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(canonical);
  if (typeof value !== "object" || value === null) return value;
  return Object.fromEntries(
    Object.entries(value)
      .filter(([, v]) => v !== undefined && v !== false)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, v]) => [k, canonical(v)]),
  );
};

export const sameCriterion = (
  a: IMSInitialFilterCriterionInput,
  b: IMSInitialFilterCriterionInput,
) => JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));
