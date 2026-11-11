// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import Operator from "./Operator";

const api = setupApiServer();

const operator = {
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
  spn: { fullName: "Ella Networks", shortName: "Ella" },
};

describe("Operator page", () => {
  it("shows the network identity without the integrations", async () => {
    api.get("/api/v1/operator", () => operator);
    renderWithProviders(<Operator />, {
      initialEntries: ["/operator"],
      auth: { role: "Admin" },
    });

    expect(await screen.findByText("001 / 01")).toBeInTheDocument();
    expect(screen.getByText("Network Name (SPN)")).toBeInTheDocument();
    expect(screen.queryByText("SMSCs")).not.toBeInTheDocument();
    expect(screen.queryByText("P-CSCF Addresses")).not.toBeInTheDocument();
    expect(screen.queryByText("Partners")).not.toBeInTheDocument();
    expect(
      api
        .requests()
        .some((r) => r.url.pathname.startsWith("/api/v1/integrations")),
    ).toBe(false);
  });
});
