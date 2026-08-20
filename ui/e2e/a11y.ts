// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";

const ENFORCED_IMPACTS = new Set(["critical", "serious"]);

export async function assertNoA11yViolations(page: Page, label: string) {
  const { violations } = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();

  const enforced = violations.filter(
    (violation) => violation.impact && ENFORCED_IMPACTS.has(violation.impact),
  );

  const summary = enforced
    .map(
      (violation) =>
        `${violation.impact}: ${violation.help} (${violation.nodes.length} node(s))\n` +
        violation.nodes
          .slice(0, 3)
          .map((node) => `    ${node.target.join(" ")}`)
          .join("\n"),
    )
    .join("\n");

  expect(
    enforced,
    `${label} has accessibility violations:\n${summary}`,
  ).toEqual([]);
}
