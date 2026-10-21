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
const SERVICES_PATH = "/api/v1/operator/ims/services";

beforeEach(() => {
  api.get(AS_PATH, () => ({ items: [] }));
  api.get(SERVICES_PATH, () => ({ items: [] }));
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

const mcpttService = {
  name: "mcptt",
  kind: "mcptt",
  application_server_name: "mcptt",
  initial_filter_criteria: [
    {
      priority: 0,
      trigger_point: {
        condition_type_cnf: false,
        spt: [{ group: [0], method: "REGISTER" }],
      },
      default_handling: "SESSION_CONTINUED",
    },
    {
      priority: 1,
      trigger_point: {
        condition_type_cnf: false,
        spt: [{ group: [0], request_uri: "sip:!.*!@mcptt.ims.example.org" }],
      },
      default_handling: "SESSION_TERMINATED",
    },
  ],
};

const customService = {
  name: "voicemail",
  kind: "custom",
  application_server_name: "vm",
  initial_filter_criteria: [
    {
      priority: 0,
      trigger_point: {
        condition_type_cnf: false,
        spt: [{ group: [0], session_case: "TERMINATING_UNREGISTERED" }],
      },
      default_handling: "SESSION_CONTINUED",
    },
  ],
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
      httpError(409, "Application server is used by an IMS service"),
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
        "Failed to delete application server: Application server is used by an IMS service",
      ),
    ).toBeInTheDocument();
  });
});

describe("Operator IMS services", () => {
  it("adds an MCPTT service on an existing application server", async () => {
    const user = userEvent.setup();
    api.get(AS_PATH, () => ({ items: [mcpttAS] }));
    api.post(SERVICES_PATH, () => ({}));
    renderIMS();

    await user.click(
      await screen.findByRole("button", { name: "Add MCPTT service" }),
    );
    await user.type(await screen.findByLabelText(/^Service Name/), "mcptt");
    await user.click(
      screen.getByRole("combobox", { name: /Application Server/ }),
    );
    await user.click(
      await screen.findByRole("option", { name: /^mcptt \(sip:/ }),
    );
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    await screen.findByText("Service added successfully.");
    expect(api.requests(AS_PATH).some((r) => r.method === "POST")).toBe(false);
    expect(
      api.requests(SERVICES_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({
      name: "mcptt",
      kind: "mcptt",
      application_server_name: "mcptt",
    });
  });

  it("creates the application server before the MCPTT service", async () => {
    const user = userEvent.setup();
    api.post(AS_PATH, () => ({}));
    api.post(SERVICES_PATH, () => ({}));
    renderIMS();

    await user.click(
      await screen.findByRole("button", { name: "Add MCPTT service" }),
    );
    await user.type(await screen.findByLabelText(/^Service Name/), "mcptt");
    expect(screen.getByRole("radio", { name: "Create new" })).toBeChecked();
    await user.type(
      screen.getByLabelText(/^Application Server Name/),
      "mcptt-as",
    );
    await user.type(
      screen.getByLabelText(/^Server Name/),
      "sip:mcptt.ims.example.org",
    );
    await user.type(
      screen.getByLabelText(/^Public Service Identity/),
      "sip:!.*!@mcptt.ims.example.org",
    );
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    await screen.findByText("Service added successfully.");
    const writes = api
      .requests()
      .filter((r) => r.method === "POST")
      .map((r) => ({ path: r.url.pathname, body: r.body }));
    expect(writes).toEqual([
      {
        path: AS_PATH,
        body: {
          name: "mcptt-as",
          server_name: "sip:mcptt.ims.example.org",
          public_service_identities: ["sip:!.*!@mcptt.ims.example.org"],
        },
      },
      {
        path: SERVICES_PATH,
        body: {
          name: "mcptt",
          kind: "mcptt",
          application_server_name: "mcptt-as",
        },
      },
    ]);
  });

  it("changes the application server of an MCPTT service", async () => {
    const user = userEvent.setup();
    const other = { ...mcpttAS, name: "other", server_name: "sip:other.org" };
    api.get(AS_PATH, () => ({ items: [mcpttAS, other] }));
    api.get(SERVICES_PATH, () => ({ items: [mcpttService] }));
    api.put(`${SERVICES_PATH}/mcptt`, () => ({}));
    renderIMS();

    await user.click(
      await screen.findByRole("button", { name: "Edit service mcptt" }),
    );
    expect(await screen.findByLabelText(/^Service Name/)).toBeDisabled();
    await user.click(
      screen.getByRole("combobox", { name: /Application Server/ }),
    );
    await user.click(await screen.findByRole("option", { name: /^other/ }));
    await user.click(screen.getByRole("button", { name: /^Update$/ }));

    await screen.findByText("Service updated successfully.");
    expect(api.lastRequest(`${SERVICES_PATH}/mcptt`)?.body).toEqual({
      kind: "mcptt",
      application_server_name: "other",
    });
  });

  it("lists custom services read-only and lets them be deleted", async () => {
    const user = userEvent.setup();
    let services: unknown[] = [mcpttService, customService];
    api.get(SERVICES_PATH, () => ({ items: services }));
    api.delete(`${SERVICES_PATH}/voicemail`, () => {
      services = [mcpttService];
      return {};
    });
    renderIMS();

    const custom = await row("voicemail");
    expect(custom.getByText("Custom")).toBeInTheDocument();
    expect(custom.getByText("vm")).toBeInTheDocument();
    expect(custom.getByText("1 filter")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit service voicemail" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Edit service mcptt" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Custom services are managed through the API/),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Delete service voicemail" }),
    );
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    await screen.findByText("Service deleted successfully.");
    expect(
      api.requests(`${SERVICES_PATH}/voicemail`).map((r) => r.method),
    ).toEqual(["DELETE"]);
  });

  it("shows the filter criteria of a service", async () => {
    const user = userEvent.setup();
    api.get(SERVICES_PATH, () => ({ items: [mcpttService] }));
    renderIMS();

    await user.click(
      await screen.findByRole("button", {
        name: "View filter criteria of mcptt",
      }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    expect(dialog.getByText("method REGISTER")).toBeInTheDocument();
    expect(
      dialog.getByText("request URI sip:!.*!@mcptt.ims.example.org"),
    ).toBeInTheDocument();
    expect(dialog.getByText("Terminate")).toBeInTheDocument();
  });

  it("hides IMS writes from a read-only user", async () => {
    api.get(AS_PATH, () => ({ items: [mcpttAS] }));
    api.get(SERVICES_PATH, () => ({ items: [mcpttService] }));
    renderIMS("Read Only");

    expect(
      await screen.findByText("sip:mcptt.ims.example.org"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add application server" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add MCPTT service" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Delete service/ }),
    ).not.toBeInTheDocument();
  });
});
