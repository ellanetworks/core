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
        applicationServerName: "as",
        defaultHandling: "SESSION_CONTINUED",
        profilePart: "UNREGISTERED",
        serviceInfo: "<info/>",
        includeRegisterRequest: false,
        includeRegisterResponse: false,
      },
    ],
    [
      "every condition type in two groups",
      {
        applicationServerName: "as",
        trigger: {
          conditionsMatch: "any",
          groups: [
            {
              conditions: [
                {
                  method: "REGISTER",
                  registrationTypes: ["INITIAL_REGISTRATION"],
                },
                { negated: true, requestURI: "^conf\\.example\\.org$" },
                { sessionCase: "ORIGINATING_CDIV" },
              ],
            },
            {
              conditions: [
                { sipHeader: { header: "^Accept-Contact$" } },
                { negated: true, sipHeader: { header: "^Privacy$" } },
                {
                  sipHeader: { header: "^From$", content: '"joe"' },
                  negated: true,
                },
                { sessionDescription: { line: "^m$", content: "^video" } },
                { negated: true, sessionDescription: { line: "^b$" } },
              ],
            },
          ],
        },
        defaultHandling: "SESSION_TERMINATED",
        includeRegisterRequest: true,
        includeRegisterResponse: true,
      },
    ],
  ])("keep %s unchanged", (_name, criterion) => {
    expect(sameCriterion(roundTrip(criterion), criterion)).toBe(true);
  });

  it("drops registration types when all of them are selected", () => {
    const values = criterionToValues(
      {
        applicationServerName: "as",
        trigger: {
          conditionsMatch: "all",
          groups: [{ conditions: [{ method: "REGISTER" }] }],
        },
        defaultHandling: "SESSION_CONTINUED",
      },
      "as",
    );
    expect(values.groups[0].conditions[0].registrationTypes).toHaveLength(3);
    expect(criterionFromValues(values).trigger).toEqual({
      conditionsMatch: "all",
      groups: [{ conditions: [{ method: "REGISTER" }] }],
    });
  });

  it("starts a new criterion with one empty condition and continue handling", () => {
    const values = criterionToValues(undefined, "mcptt");
    expect(values).toMatchObject({
      applicationServerName: "mcptt",
      defaultHandling: "SESSION_CONTINUED",
      always: false,
      conditionsMatch: "all",
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
          applicationServerName: "as",
          includeRegisterRequest: false,
          defaultHandling: "SESSION_CONTINUED",
        },
        {
          defaultHandling: "SESSION_CONTINUED",
          applicationServerName: "as",
        },
      ),
    ).toBe(true);
    expect(
      sameCriterion(
        { applicationServerName: "as" },
        { applicationServerName: "other" },
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
        conditionsMatch: "all",
        groups: [
          {
            conditions: [
              { sessionCase: "ORIGINATING_REGISTERED" },
              {
                sipHeader: {
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
        conditionsMatch: "any",
        groups: [
          { conditions: [{ method: "INVITE" }, { method: "MESSAGE" }] },
          {
            conditions: [{ negated: true, sipHeader: { header: "^Privacy$" } }],
          },
        ],
      },
      "(method is INVITE or method is MESSAGE) and header ^Privacy$ absent",
    ],
  ])("describes %s", (_name, trigger, want) => {
    expect(describeTrigger(trigger)).toBe(want);
  });
});
