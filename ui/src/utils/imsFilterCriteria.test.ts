// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import type { IMSInitialFilterCriterionInput } from "@/queries/ims";
import { describeTrigger } from "@/queries/ims";
import {
  criterionFromValues,
  criterionToValues,
  sameCriterion,
} from "./imsFilterCriteria";

const roundTrip = (c: IMSInitialFilterCriterionInput) =>
  criterionFromValues(criterionToValues(c, "other"));

describe("criterionToValues and criterionFromValues", () => {
  it.each<[string, IMSInitialFilterCriterionInput]>([
    [
      "an unconditional criterion",
      {
        application_server_name: "as",
        default_handling: "SESSION_CONTINUED",
        profile_part: "UNREGISTERED",
        service_info: "<info/>",
        include_register_request: false,
        include_register_response: false,
      },
    ],
    [
      "every condition type in two groups",
      {
        application_server_name: "as",
        trigger: {
          conditions_match: "any",
          groups: [
            {
              conditions: [
                {
                  method: "REGISTER",
                  registration_types: ["INITIAL_REGISTRATION"],
                },
                { negated: true, request_uri: "^conf\\.example\\.org$" },
                { session_case: "ORIGINATING_CDIV" },
              ],
            },
            {
              conditions: [
                { sip_header: { header: "^Accept-Contact$" } },
                { negated: true, sip_header: { header: "^Privacy$" } },
                {
                  sip_header: { header: "^From$", content: '"joe"' },
                  negated: true,
                },
                { session_description: { line: "^m$", content: "^video" } },
                { negated: true, session_description: { line: "^b$" } },
              ],
            },
          ],
        },
        default_handling: "SESSION_TERMINATED",
        include_register_request: true,
        include_register_response: true,
      },
    ],
  ])("keep %s unchanged", (_name, criterion) => {
    expect(sameCriterion(roundTrip(criterion), criterion)).toBe(true);
  });

  it("drops registration types when all of them are selected", () => {
    const values = criterionToValues(
      {
        application_server_name: "as",
        trigger: {
          conditions_match: "all",
          groups: [{ conditions: [{ method: "REGISTER" }] }],
        },
        default_handling: "SESSION_CONTINUED",
      },
      "as",
    );
    expect(values.groups[0].conditions[0].registration_types).toHaveLength(3);
    expect(criterionFromValues(values).trigger).toEqual({
      conditions_match: "all",
      groups: [{ conditions: [{ method: "REGISTER" }] }],
    });
  });

  it("starts a new criterion with one empty condition and continue handling", () => {
    const values = criterionToValues(undefined, "mcptt");
    expect(values).toMatchObject({
      application_server_name: "mcptt",
      default_handling: "SESSION_CONTINUED",
      always: false,
      conditions_match: "all",
    });
    expect(values.groups).toHaveLength(1);
    expect(values.groups[0].conditions).toHaveLength(1);
  });
});

describe("sameCriterion", () => {
  it("ignores key order and false flags", () => {
    expect(
      sameCriterion(
        {
          application_server_name: "as",
          include_register_request: false,
          default_handling: "SESSION_CONTINUED",
        },
        {
          default_handling: "SESSION_CONTINUED",
          application_server_name: "as",
        },
      ),
    ).toBe(true);
    expect(
      sameCriterion(
        { application_server_name: "as" },
        { application_server_name: "other" },
      ),
    ).toBe(false);
  });
});

describe("describeTrigger", () => {
  it.each<[string, IMSInitialFilterCriterionInput["trigger"], string]>([
    ["no trigger", undefined, "Always"],
    [
      "one group of all",
      {
        conditions_match: "all",
        groups: [
          {
            conditions: [
              { session_case: "ORIGINATING_REGISTERED" },
              {
                sip_header: {
                  header: "^P-Asserted-Service$",
                  content: "mcptt",
                },
              },
            ],
          },
        ],
      },
      "Originating (registered) and header ^P-Asserted-Service$ matches mcptt",
    ],
    [
      "groups of any",
      {
        conditions_match: "any",
        groups: [
          { conditions: [{ method: "INVITE" }, { method: "MESSAGE" }] },
          {
            conditions: [
              { negated: true, sip_header: { header: "^Privacy$" } },
            ],
          },
        ],
      },
      "(method is INVITE or method is MESSAGE) and header ^Privacy$ absent",
    ],
  ])("describes %s", (_name, trigger, want) => {
    expect(describeTrigger(trigger)).toBe(want);
  });
});
