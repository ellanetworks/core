// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DEFAULT_PRODUCT_NAME, PRODUCT, logoAlt } from "@/utils/product";
import Footer from "./Footer";
import { ipv6PoolHelperText } from "./dataNetworkForm";

afterEach(() => {
  PRODUCT.name = DEFAULT_PRODUCT_NAME;
});

describe("product name", () => {
  it("propagates to every derived string", () => {
    PRODUCT.name = "Northwind";

    expect(logoAlt()).toBe("Northwind Logo");
    expect(ipv6PoolHelperText()).toContain("Northwind");
  });
});

describe("Footer", () => {
  it("shows the vendor identity, not the product name", () => {
    PRODUCT.name = "Northwind";

    render(<Footer />);

    expect(screen.getByText(/Ella Networks Inc\./)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "ellanetworks.com" }),
    ).toHaveAttribute("href", "https://ellanetworks.com");
    expect(screen.queryByText(/Northwind/)).not.toBeInTheDocument();
  });
});
