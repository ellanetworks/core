// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import Logo, { MARK_MAX_SIZE } from "@/components/Logo";

describe("Logo", () => {
  it("uses the simplified mark up to the threshold", () => {
    render(<Logo width={MARK_MAX_SIZE} height={MARK_MAX_SIZE} />);
    expect(screen.getByRole("img")).toHaveAttribute("src", "/logo-mark.svg");
  });

  it("uses the full logo above the threshold", () => {
    render(<Logo width={MARK_MAX_SIZE + 1} height={MARK_MAX_SIZE + 1} />);
    expect(screen.getByRole("img")).toHaveAttribute("src", "/logo.svg");
  });
});
