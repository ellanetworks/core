// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DEFAULT_PRODUCT_NAME, PRODUCT, logoAlt } from "@/utils/product";
import Footer from "./Footer";
import ProductTitle from "./ProductTitle";
import { ipv6PoolHelperText } from "./dataNetworkForm";

afterEach(() => {
  PRODUCT.name = DEFAULT_PRODUCT_NAME;
  PRODUCT.showProductName = true;
  PRODUCT.poweredBy = false;
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

describe("ProductTitle", () => {
  it("shows only the product name for the default branding", () => {
    render(<ProductTitle />);

    expect(screen.getByText(DEFAULT_PRODUCT_NAME)).toBeInTheDocument();
    expect(screen.queryByText(/Powered by/)).not.toBeInTheDocument();
  });

  it("credits Ella Networks for a custom branding", () => {
    PRODUCT.name = "Northwind";
    PRODUCT.poweredBy = true;

    render(<ProductTitle />);

    expect(screen.getByText("Northwind")).toBeInTheDocument();
    expect(screen.getByText("Powered by Ella Networks")).toBeInTheDocument();
  });

  it("drops the name when the logo already shows it", () => {
    PRODUCT.name = "Northwind";
    PRODUCT.showProductName = false;
    PRODUCT.poweredBy = true;

    render(<ProductTitle />);

    expect(screen.queryByText("Northwind")).not.toBeInTheDocument();
    expect(screen.getByText("Powered by Ella Networks")).toBeInTheDocument();
    expect(logoAlt()).toBe("Northwind");
  });
});
