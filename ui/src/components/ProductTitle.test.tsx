// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DEFAULT_PRODUCT_NAME, PRODUCT } from "@/utils/product";
import ProductTitle from "./ProductTitle";
import Footer from "./Footer";

afterEach(() => {
  PRODUCT.name = DEFAULT_PRODUCT_NAME;
});

describe("ProductTitle", () => {
  it("shows the product name alone under the default branding", () => {
    render(<ProductTitle />);

    expect(screen.getByText(DEFAULT_PRODUCT_NAME)).toBeInTheDocument();
    expect(screen.queryByText(/Powered by/)).not.toBeInTheDocument();
  });

  it("credits the vendor when the product is renamed", () => {
    PRODUCT.name = "Kygo";

    render(<ProductTitle />);

    expect(screen.getByText("Kygo")).toBeInTheDocument();
    expect(screen.getByText("Powered by Ella Networks")).toBeInTheDocument();
  });
});

describe("Footer", () => {
  it("keeps the vendor identity when the product is renamed", () => {
    PRODUCT.name = "Kygo";

    render(<Footer />);

    expect(screen.getByText(/Ella Networks Inc\./)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "ellanetworks.com" }),
    ).toHaveAttribute("href", "https://ellanetworks.com");
    expect(screen.queryByText(/Kygo/)).not.toBeInTheDocument();
  });
});
