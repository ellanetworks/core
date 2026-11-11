// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Outlet, Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { stubDiameterStatus } from "@/test/diameter";
import SMSTab from "./SMSTab";

const api = setupApiServer();

beforeEach(() => {
  stubDiameterStatus(api);
});

const SMS_PATH = "/api/v1/integrations/sms";
const SMSCS_PATH = "/api/v1/integrations/sms/smscs";

const nodeA = {
  originHost: "smsc-a.example.org",
  address: "2001:db8::10",
  port: 3868,
  status: {
    state: "open",
    originHost: "smsc-a.example.org",
    realm: "example.org",
    since: "2026-09-29T16:00:00Z",
  },
};

const nodeB = {
  originHost: "smsc-b.example.org",
  address: "192.0.2.20",
  port: 3869,
  status: { state: "down", since: "2026-09-29T16:00:00Z" },
};

const main = {
  name: "main",
  serviceCentres: ["+15550000000"],
  nodes: [nodeA],
};

const backup = {
  name: "backup",
  serviceCentres: ["+15550000001", "+15550000002"],
  nodes: [{ ...nodeA, address: "2001:db8::11" }, nodeB],
};

const ready = { servingNodeNumber: "+15550001111" };

const row = async (text: string) => {
  const cell = await screen.findByText(text);
  return within(cell.closest("tr")!);
};

const renderTab = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route element={<Outlet context={{ canEdit: role !== "Read Only" }} />}>
        <Route path="/" element={<SMSTab />} />
      </Route>
    </Routes>,
    { auth: { role } },
  );

describe("Integrations SMS tab", () => {
  it("shows an unset SMS number and no SMSCs", async () => {
    api.get(SMS_PATH, () => ({ servingNodeNumber: "" }));
    api.get(SMSCS_PATH, () => ({ items: [] }));
    renderTab();

    expect(
      await (await row("Serving Node Number")).findByText("N/A"),
    ).toBeInTheDocument();
    expect(screen.getByText("SMSCs")).toBeInTheDocument();
    expect(await screen.findByText("No SMSCs yet.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Edit serving node number" }),
    ).toBeInTheDocument();
  });

  it("lists each SMSC with the status of its nodes", async () => {
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({
      items: [main, backup],
    }));
    renderTab();

    expect(await screen.findByText("+15550001111")).toBeInTheDocument();

    const a = await row("main");
    expect(a.getByText("+15550000000")).toBeInTheDocument();
    expect(a.getByText("[2001:db8::10]:3868")).toBeInTheDocument();
    expect(a.getByText("Connected")).toBeInTheDocument();

    const b = await row("backup");
    expect(b.getByText("+15550000001, +15550000002")).toBeInTheDocument();
    expect(b.getByText("[2001:db8::11]:3868")).toBeInTheDocument();
    expect(b.getByText("192.0.2.20:3869")).toBeInTheDocument();
    expect(b.getByText("1 of 2 connected")).toBeInTheDocument();
  });

  it("lists the state of each node of an SMSC on hover", async () => {
    const user = userEvent.setup();
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: [backup] }));
    renderTab();

    await user.hover((await row("backup")).getByText("1 of 2 connected"));

    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent("[2001:db8::11]:3868: Connected");
    expect(tooltip).toHaveTextContent("192.0.2.20:3869: Disconnected");
  });

  it("shows why a node is disconnected", async () => {
    const user = userEvent.setup();
    const error =
      "peer answered as smsc.example.org, expected smsc-b.example.org";
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({
      items: [
        { ...main, nodes: [{ ...nodeB, status: { ...nodeB.status, error } }] },
      ],
    }));
    renderTab();

    await user.hover((await row("main")).getByText("Disconnected"));

    expect(await screen.findByRole("tooltip")).toHaveTextContent(error);
  });

  it("shows a node this node has not reported as connecting", async () => {
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({
      items: [{ ...main, nodes: [{ ...nodeA, status: undefined }] }],
    }));
    renderTab();

    expect((await row("main")).getByText("Connecting")).toBeInTheDocument();
  });

  it("adds an SMSC", async () => {
    const user = userEvent.setup();
    let smscs: unknown[] = [];
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: smscs }));
    api.post(SMSCS_PATH, () => {
      smscs = [{ ...main, nodes: [nodeB] }];
      return {};
    });
    renderTab();

    await user.click(await screen.findByRole("button", { name: "Add SMSC" }));
    await user.type(await screen.findByLabelText(/^Name/), "main");
    await user.type(
      screen.getByLabelText(/^Service Centre Addresses/),
      "+15550000000",
    );
    await user.type(
      screen.getByLabelText(/^Diameter peer 1 address/),
      "192.0.2.20",
    );
    await user.type(
      screen.getByLabelText(/^Diameter peer 1 Origin-Host/),
      "smsc-b.example.org",
    );
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    expect(await screen.findByText("192.0.2.20:3869")).toBeInTheDocument();
    expect(
      api.requests(SMSCS_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({
      name: "main",
      serviceCentres: ["+15550000000"],
      nodes: [
        {
          originHost: "smsc-b.example.org",
          address: "192.0.2.20",
          port: 3868,
        },
      ],
    });
  });

  it("warns before deleting the only SMSC", async () => {
    const user = userEvent.setup();
    let smscs: unknown[] = [main];
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: smscs }));
    api.delete(`${SMSCS_PATH}/main`, () => {
      smscs = [];
      return {};
    });
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Delete SMSC main" }),
    );
    expect(
      await screen.findByText(/subscribers lose SMS until you add another/),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(await screen.findByText("No SMSCs yet.")).toBeInTheDocument();
    expect(api.requests(`${SMSCS_PATH}/main`)).toHaveLength(1);
  });

  it("lets a read-only user see but not change SMS", async () => {
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: [main] }));
    renderTab("Read Only");

    expect(await screen.findByText("[2001:db8::10]:3868")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit serving node number" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add SMSC" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Delete SMSC/ }),
    ).not.toBeInTheDocument();
  });
});

describe("Integrations SMS tab Diameter realm", () => {
  it("shows the realm to enter on the SMSC", async () => {
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: [] }));
    renderTab();

    expect(
      await (
        await row("Diameter Realm")
      ).findByText("epc.mnc001.mcc001.3gppnetwork.org"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy Diameter Realm" }),
    ).toBeInTheDocument();
  });

  it("puts the status right before the actions", async () => {
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: [main] }));
    renderTab();

    await row("main");
    const headers = within(screen.getAllByRole("table")[1])
      .getAllByRole("cell")
      .slice(0, 5)
      .map((c) => c.textContent);
    expect(headers).toEqual([
      "Name",
      "Service Centre Addresses",
      "Diameter Peers",
      "Status",
      "Actions",
    ]);
  });
});

describe("Integrations SMS readiness", () => {
  it("warns without a serving node number and without SMSCs", async () => {
    api.get(SMS_PATH, () => ({ servingNodeNumber: "" }));
    api.get(SMSCS_PATH, () => ({ items: [] }));
    renderTab();

    expect(
      await screen.findByText(
        /No serving node number is set: messages sent to your subscribers cannot be delivered/,
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        /No SMSC is configured: subscribers cannot send or receive SMS/,
      ),
    ).toBeInTheDocument();
  });

  it("shows no warning when SMS is ready", async () => {
    api.get(SMS_PATH, () => ready);
    api.get(SMSCS_PATH, () => ({ items: [main] }));
    renderTab();

    await row("main");
    await (await row("Serving Node Number")).findByText("+15550001111");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
