// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import Operator from "./Operator";

const api = setupApiServer();

const PEERS_PATH = "/api/v1/operator/sms/smsc-peers";
const AS_PATH = "/api/v1/operator/ims/application-servers";
const SP_PATH = "/api/v1/operator/ims/service-profiles";

beforeEach(() => {
  api.get(AS_PATH, () => ({ items: [] }));
  api.get(SP_PATH, () => ({ items: [] }));
});

const operator = (
  sms: Record<string, unknown>,
  pcscfAddresses: string[] = [],
) => ({
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
  spn: { fullName: "Ella Networks", shortName: "Ella" },
  sms,
  ims: { pcscfAddresses, voiceStatus: "no_pcscf" },
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

describe("Operator IMS section", () => {
  it("shows the P-CSCF addresses", async () => {
    api.get("/api/v1/operator", () =>
      operator(ready, ["10.6.0.5", "2001:db8::5"]),
    );
    api.get(PEERS_PATH, () => ({ items: [] }));
    renderOperator();

    expect(
      await (await row("P-CSCF Addresses")).findByText("10.6.0.5, 2001:db8::5"),
    ).toBeInTheDocument();
  });

  it("updates the P-CSCF addresses", async () => {
    const user = userEvent.setup();
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: [] }));
    api.put("/api/v1/operator/ims", () => ({}));
    renderOperator();

    await user.click(
      await screen.findByRole("button", { name: "Edit P-CSCF addresses" }),
    );
    await user.type(
      await screen.findByLabelText(/^Addresses/),
      "10.6.0.5, 2001:db8::5",
    );
    await user.click(screen.getByRole("button", { name: /^Update$/ }));

    await screen.findByText("P-CSCF addresses updated successfully.");
    expect(api.lastRequest("/api/v1/operator/ims")?.body).toEqual({
      pcscfAddresses: ["10.6.0.5", "2001:db8::5"],
    });
  });

  it("rejects malformed and duplicate P-CSCF addresses", async () => {
    const user = userEvent.setup();
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: [] }));
    renderOperator();

    await user.click(
      await screen.findByRole("button", { name: "Edit P-CSCF addresses" }),
    );
    const input = await screen.findByLabelText(/^Addresses/);

    await user.type(input, "10.6.0.300");
    await user.tab();
    expect(
      await screen.findByText("10.6.0.300 is not an IPv4 or IPv6 address"),
    ).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, "10.6.0.5, 10.6.0.5");
    await user.tab();
    expect(
      await screen.findByText("10.6.0.5 is listed more than once"),
    ).toBeInTheDocument();
  });

  it("hides the P-CSCF edit from a read-only user", async () => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(PEERS_PATH, () => ({ items: [] }));
    renderOperator("Read Only");

    expect(await screen.findByText("P-CSCF Addresses")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit P-CSCF addresses" }),
    ).not.toBeInTheDocument();
  });
});

const mcpttAS = {
  name: "mcptt",
  server_name: "sip:mcptt.ims.example.org",
  public_service_identities: ["sip:!.*!@mcptt.ims.example.org"],
};

const renderIMS = (role = "Admin") => {
  api.get("/api/v1/operator", () => operator(ready));
  api.get(PEERS_PATH, () => ({ items: [] }));
  renderOperator(role);
};

describe("Operator IMS application servers", () => {
  it("adds an application server with public service identities", async () => {
    const user = userEvent.setup();
    let servers: unknown[] = [];
    api.get(AS_PATH, () => ({ items: servers }));
    api.post(AS_PATH, () => {
      servers = [mcpttAS];
      return {};
    });
    renderIMS();

    expect(
      await screen.findByText("No application servers yet."),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Add application server" }),
    );
    await user.type(await screen.findByLabelText(/^Name/), "mcptt");
    await user.type(
      screen.getByLabelText(/^Server Name/),
      "sip:mcptt.ims.example.org",
    );
    await user.click(screen.getByRole("button", { name: "Add Identity" }));
    await user.type(
      screen.getByLabelText(/^Identity 1/),
      "sip:!.*!@mcptt.ims.example.org",
    );
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    await screen.findByText("Application server added successfully.");
    expect(
      api.requests(AS_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual(mcpttAS);
  });

  it("adds an application server without identities", async () => {
    const user = userEvent.setup();
    api.post(AS_PATH, () => ({}));
    renderIMS();

    await user.click(
      await screen.findByRole("button", { name: "Add application server" }),
    );
    await user.type(await screen.findByLabelText(/^Name/), "conf");
    await user.type(
      screen.getByLabelText(/^Server Name/),
      "sip:conf.ims.example.org",
    );
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    await screen.findByText("Application server added successfully.");
    expect(
      api.requests(AS_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({
      name: "conf",
      server_name: "sip:conf.ims.example.org",
      public_service_identities: [],
    });
  });

  it("rejects a server name that is not a SIP URI", async () => {
    const user = userEvent.setup();
    renderIMS();

    await user.click(
      await screen.findByRole("button", { name: "Add application server" }),
    );
    await user.type(await screen.findByLabelText(/^Name/), "conf");
    await user.type(screen.getByLabelText(/^Server Name/), "conf.example.org");
    await user.tab();

    expect(await screen.findByText(/Must be a SIP URI/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Add$/ })).toBeDisabled();
  });

  it("edits an application server", async () => {
    const user = userEvent.setup();
    api.get(AS_PATH, () => ({ items: [mcpttAS] }));
    api.put(`${AS_PATH}/mcptt`, () => ({}));
    renderIMS();

    await user.click(
      await screen.findByRole("button", {
        name: "Edit application server mcptt",
      }),
    );
    expect(await screen.findByLabelText(/^Name/)).toBeDisabled();
    const server = screen.getByLabelText(/^Server Name/);
    await user.clear(server);
    await user.type(server, "sip:as.ims.example.org");
    await user.click(screen.getByRole("button", { name: "Remove identity 1" }));
    await user.click(screen.getByRole("button", { name: /^Update$/ }));

    await screen.findByText("Application server updated successfully.");
    expect(api.lastRequest(`${AS_PATH}/mcptt`)?.body).toEqual({
      server_name: "sip:as.ims.example.org",
      public_service_identities: [],
    });
  });

  it("deletes an application server", async () => {
    const user = userEvent.setup();
    let servers: unknown[] = [mcpttAS];
    api.get(AS_PATH, () => ({ items: servers }));
    api.delete(`${AS_PATH}/mcptt`, () => {
      servers = [];
      return {};
    });
    renderIMS();

    await user.click(
      await screen.findByRole("button", {
        name: "Delete application server mcptt",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(
      await screen.findByText("No application servers yet."),
    ).toBeInTheDocument();
    expect(api.requests(`${AS_PATH}/mcptt`).map((r) => r.method)).toEqual([
      "DELETE",
    ]);
  });

  it("reports an application server still in use", async () => {
    const user = userEvent.setup();
    api.get(AS_PATH, () => ({ items: [mcpttAS] }));
    api.delete(`${AS_PATH}/mcptt`, () =>
      httpError(409, "Application server is used by a service profile"),
    );
    renderIMS();

    await user.click(
      await screen.findByRole("button", {
        name: "Delete application server mcptt",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(
      await screen.findByText(
        "Failed to delete application server: Application server is used by a service profile",
      ),
    ).toBeInTheDocument();
  });
});

const mcpttSP = {
  name: "mcptt",
  initial_filter_criteria: [
    {
      priority: 1,
      application_server_name: "mcptt",
      default_handling: "SESSION_TERMINATED",
    },
    {
      priority: 0,
      application_server_name: "ip-sm-gw",
      default_handling: "SESSION_CONTINUED",
    },
  ],
};

describe("Operator IMS service profiles", () => {
  it("lists each service profile with its application servers", async () => {
    api.get(SP_PATH, () => ({
      items: [mcpttSP, { name: "empty", initial_filter_criteria: [] }],
    }));
    renderIMS();

    const mcpttRow = (await screen.findByText("ip-sm-gw, mcptt")).closest(
      "tr",
    )!;
    expect(mcpttRow).toHaveTextContent("mcptt");
    expect(screen.getByText("empty").closest("tr")!).toHaveTextContent("None");
  });

  it("adds a service profile", async () => {
    const user = userEvent.setup();
    api.post(SP_PATH, () => ({}));
    renderIMS();

    await user.click(
      await screen.findByRole("button", { name: "Add service profile" }),
    );
    const dialog = within(
      await screen.findByRole("dialog", { name: "Add Service Profile" }),
    );
    await user.type(dialog.getByRole("textbox", { name: "Name" }), "volte");
    await user.click(dialog.getByRole("button", { name: "Add" }));

    await screen.findByText("Service profile added successfully.");
    expect(
      api.requests(SP_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({ name: "volte", initial_filter_criteria: [] });
  });

  it("reports a service profile still assigned", async () => {
    const user = userEvent.setup();
    api.get(SP_PATH, () => ({ items: [mcpttSP] }));
    api.delete(`${SP_PATH}/mcptt`, () =>
      httpError(409, "Service profile is assigned to a profile"),
    );
    renderIMS();

    await user.click(
      await screen.findByRole("button", {
        name: "Delete service profile mcptt",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(
      await screen.findByText(
        "Failed to delete service profile: Service profile is assigned to a profile",
      ),
    ).toBeInTheDocument();
  });

  it("hides service profile writes from a read-only user", async () => {
    api.get(SP_PATH, () => ({ items: [mcpttSP] }));
    renderIMS("Read Only");

    expect(await screen.findByText("ip-sm-gw, mcptt")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add service profile" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Edit service profile/ }),
    ).not.toBeInTheDocument();
  });
});

describe("Operator IMS without services", () => {
  it("shows application servers but no services", async () => {
    api.get(AS_PATH, () => ({ items: [mcpttAS] }));
    renderIMS();

    expect(
      await screen.findByText("sip:mcptt.ims.example.org"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Services")).not.toBeInTheDocument();
    expect(screen.queryByText("No services yet.")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /service$/i }),
    ).not.toBeInTheDocument();
    expect(
      api.requests().some((r) => r.url.pathname.includes("/ims/services")),
    ).toBe(false);
  });

  it("hides IMS writes from a read-only user", async () => {
    api.get(AS_PATH, () => ({ items: [mcpttAS] }));
    renderIMS("Read Only");

    expect(
      await screen.findByText("sip:mcptt.ims.example.org"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add application server" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Edit application server/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Delete application server/ }),
    ).not.toBeInTheDocument();
  });
});
