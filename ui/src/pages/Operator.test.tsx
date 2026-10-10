// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import Operator from "./Operator";

const api = setupApiServer();

const PEERS_PATH = "/api/v1/operator/sms/smsc-peers";

const operator = (sms: Record<string, unknown>) => ({
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
  spn: { fullName: "Ella Networks", shortName: "Ella" },
  sms,
});

const peerA = {
  id: "0190a000-0000-7000-8000-00000000000a",
  diameterIdentity: "smsc-a.example.org",
  address: "2001:db8::10",
  port: 3868,
  serviceCentres: ["+15550000000"],
  status: {
    state: "open",
    host: "smsc.example.org",
    realm: "example.org",
    since: "2026-09-29T16:00:00Z",
  },
};

const peerB = {
  id: "0190a000-0000-7000-8000-00000000000b",
  diameterIdentity: "smsc-b.example.org",
  address: "192.0.2.20",
  port: 3869,
  serviceCentres: ["+15550000001", "+15550000002"],
  status: { state: "down", since: "2026-09-29T16:00:00Z" },
};

const ready = { smsNumber: "+15550001111" };

const row = async (text: string) => {
  const cell = await screen.findByText(text);
  return within(cell.closest("tr")!);
};

const renderOperator = (role = "Admin") =>
  renderWithProviders(<Operator />, {
    initialEntries: ["/operator"],
    auth: { role },
  });

describe("Operator SMS section", () => {
  it("shows an unset SMS number and no service centers", async () => {
    api.get("/api/v1/operator", () => operator({ smsNumber: "" }));
    api.get(PEERS_PATH, () => ({ items: [] }));
    renderOperator();

    expect(
      await (await row("SMS Number")).findByText("N/A"),
    ).toBeInTheDocument();
    expect(screen.getByText("Service Centers")).toBeInTheDocument();
    expect(
      await screen.findByText("No service centers yet."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Edit SMS number" }),
    ).toBeInTheDocument();
  });

  it("lists each service center with its status", async () => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: [peerA, peerB] }));
    renderOperator();

    expect(await screen.findByText("+15550001111")).toBeInTheDocument();

    const a = await row("[2001:db8::10]:3868");
    expect(a.getByText("Connected")).toBeInTheDocument();
    expect(a.getByText("+15550000000")).toBeInTheDocument();

    const b = await row("192.0.2.20:3869");
    expect(b.getByText("Disconnected")).toBeInTheDocument();
    expect(b.getByText("+15550000001, +15550000002")).toBeInTheDocument();
  });

  it("shows why a service center is disconnected", async () => {
    const user = userEvent.setup();
    const error =
      "peer answered as smsc.example.org, expected smsc-b.example.org";
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({
      items: [{ ...peerB, status: { ...peerB.status, error } }],
    }));
    renderOperator();

    await user.hover(
      await (await row("192.0.2.20:3869")).findByText("Disconnected"),
    );

    expect(await screen.findByRole("tooltip")).toHaveTextContent(error);
  });

  it("shows a peer this node has not reported as connecting", async () => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: [{ ...peerA, status: undefined }] }));
    renderOperator();

    expect(
      await (await row("[2001:db8::10]:3868")).findByText("Connecting"),
    ).toBeInTheDocument();
  });

  it("adds a service center", async () => {
    const user = userEvent.setup();
    let peers: unknown[] = [];
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: peers }));
    api.post(PEERS_PATH, () => {
      peers = [peerB];
      return peerB;
    });
    renderOperator();

    await user.click(
      await screen.findByRole("button", { name: "Add service center" }),
    );
    await user.type(
      await screen.findByLabelText(/Diameter Identity/),
      "smsc-b.example.org",
    );
    await user.type(screen.getByLabelText(/^Address/), "192.0.2.20");
    await user.type(screen.getByLabelText(/^Numbers/), "+15550000001");
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    expect(await screen.findByText("192.0.2.20:3869")).toBeInTheDocument();
    expect(
      api.requests(PEERS_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({
      diameterIdentity: "smsc-b.example.org",
      address: "192.0.2.20",
      port: 3868,
      serviceCentres: ["+15550000001"],
    });
  });

  it("warns before deleting the only service center", async () => {
    const user = userEvent.setup();
    let peers: unknown[] = [peerA];
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: peers }));
    api.delete(`${PEERS_PATH}/${peerA.id}`, () => {
      peers = [];
      return {};
    });
    renderOperator();

    await user.click(
      await screen.findByRole("button", {
        name: "Delete service center [2001:db8::10]:3868",
      }),
    );
    expect(
      await screen.findByText(/subscribers lose SMS until you add another/),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(
      await screen.findByText("No service centers yet."),
    ).toBeInTheDocument();
    expect(api.requests(`${PEERS_PATH}/${peerA.id}`)).toHaveLength(1);
  });

  it("lets a read-only user see but not change SMS", async () => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: [peerA] }));
    renderOperator("Read Only");

    expect(await screen.findByText("[2001:db8::10]:3868")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit SMS number" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add service center" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Delete service center/ }),
    ).not.toBeInTheDocument();
  });
});
