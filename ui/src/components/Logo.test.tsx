// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import Logo from "@/components/Logo";

describe("Logo", () => {
  it("renders the mark", () => {
    render(<Logo width={50} height={50} />);
    expect(screen.getByRole("img")).toHaveAttribute("src", "/logo-mark.svg");
  });
});
