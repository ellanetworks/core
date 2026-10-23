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
  | "method"
  | "request_uri"
  | "sip_header"
  | "session_case"
  | "session_description";

export type ConditionOperator = "is" | "is_not" | "present" | "absent";

export interface ConditionValues {
  type: ConditionType;
  operator: ConditionOperator;
  method: string;
  registration_types: IMSRegistrationType[];
  request_uri: string;
  header: string;
  header_content: string;
  session_case: IMSSessionCase;
  line: string;
  line_content: string;
}

export interface CriterionValues {
  application_server_name: string;
  default_handling: "SESSION_CONTINUED" | "SESSION_TERMINATED";
  always: boolean;
  conditions_match: IMSConditionsMatch;
  groups: { conditions: ConditionValues[] }[];
  profile_part: "BOTH" | "REGISTERED" | "UNREGISTERED";
  service_info: string;
  include_register_request: boolean;
  include_register_response: boolean;
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
  registration_types: [...allRegistrationTypes],
  request_uri: "",
  header: "",
  header_content: "",
  session_case: "ORIGINATING_REGISTERED",
  line: "",
  line_content: "",
});

const conditionToValues = (c: IMSCondition): ConditionValues => {
  const values = emptyCondition();
  values.operator = c.negated ? "is_not" : "is";
  if (c.method !== undefined) {
    values.method = c.method;
    if (c.registration_types?.length)
      values.registration_types = [...c.registration_types];
  } else if (c.request_uri !== undefined) {
    values.type = "request_uri";
    values.request_uri = c.request_uri;
  } else if (c.sip_header !== undefined) {
    values.type = "sip_header";
    values.header = c.sip_header.header;
    values.header_content = c.sip_header.content ?? "";
    if (c.sip_header.content === undefined)
      values.operator = c.negated ? "absent" : "present";
  } else if (c.session_case !== undefined) {
    values.type = "session_case";
    values.session_case = c.session_case;
  } else if (c.session_description !== undefined) {
    values.type = "session_description";
    values.line = c.session_description.line;
    values.line_content = c.session_description.content ?? "";
    if (c.session_description.content === undefined)
      values.operator = c.negated ? "absent" : "present";
  }
  return values;
};

export const criterionToValues = (
  ifc: IMSInitialFilterCriterionInput | undefined,
  defaultServer: string,
): CriterionValues => ({
  application_server_name: ifc?.application_server_name ?? defaultServer,
  default_handling: ifc?.default_handling ?? "SESSION_CONTINUED",
  always: !!ifc && !ifc.trigger,
  conditions_match: ifc?.trigger?.conditions_match ?? "all",
  groups: ifc?.trigger?.groups.length
    ? ifc.trigger.groups.map((g) => ({
        conditions: g.conditions.map(conditionToValues),
      }))
    : [{ conditions: [emptyCondition()] }],
  profile_part: ifc?.profile_part ?? "BOTH",
  service_info: ifc?.service_info ?? "",
  include_register_request: ifc?.include_register_request ?? false,
  include_register_response: ifc?.include_register_response ?? false,
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
        v.registration_types.length < allRegistrationTypes.length
      )
        out.registration_types = allRegistrationTypes.filter((t) =>
          v.registration_types.includes(t),
        );
      break;
    case "request_uri":
      out.request_uri = v.request_uri;
      break;
    case "sip_header":
      out.sip_header = withContent
        ? { header: v.header, content: v.header_content }
        : { header: v.header };
      break;
    case "session_case":
      out.session_case = v.session_case;
      break;
    case "session_description":
      out.session_description = withContent
        ? { line: v.line, content: v.line_content }
        : { line: v.line };
      break;
  }
  return out;
};

export const criterionFromValues = (
  v: CriterionValues,
): IMSInitialFilterCriterionInput => ({
  application_server_name: v.application_server_name,
  ...(v.always
    ? {}
    : {
        trigger: {
          conditions_match: v.conditions_match,
          groups: v.groups.map((g) => ({
            conditions: g.conditions.map(conditionFromValues),
          })),
        },
      }),
  default_handling: v.default_handling,
  ...(v.profile_part !== "BOTH" ? { profile_part: v.profile_part } : {}),
  ...(v.service_info ? { service_info: v.service_info } : {}),
  include_register_request: v.include_register_request,
  include_register_response: v.include_register_response,
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
