// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Outlet, Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import { diameterStatus, stubDiameterStatus } from "@/test/diameter";
import IMSTab from "./IMSTab";

const api = setupApiServer();

beforeEach(() => {
  stubDiameterStatus(api);
});

const IMS_PATH = "/api/v1/integrations/ims";
const IMS_PEERS_PATH = "/api/v1/integrations/ims/allowed-peers";
const AS_PATH = "/api/v1/integrations/ims/application-servers";
const SP_PATH = "/api/v1/integrations/ims/service-profiles";

const imsSettings = (
  pcscfAddresses: string[] = [],
  overrides: Record<string, unknown> = {},
) => ({
  pcscfAddresses,
  voiceStatus: "no_pcscf",
  homeDomain: "",
  derivedHomeDomain: "ims.mnc001.mcc001.3gppnetwork.org",
  restrictPeers: true,
  advertiseVoice: true,
  ...overrides,
});

beforeEach(() => {
  api.get(IMS_PATH, () => imsSettings());
  api.get(IMS_PEERS_PATH, () => ({ items: [] }));
  api.get(AS_PATH, () => ({ items: [] }));
  api.get(SP_PATH, () => ({ items: [] }));
});

const row = async (text: string) => {
  const cell = await screen.findByText(text);
  return within(cell.closest("tr")!);
};

const putBody = () =>
  api.requests(IMS_PATH).find((r) => r.method === "PUT")?.body;

const renderTab = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route element={<Outlet context={{ canEdit: role !== "Read Only" }} />}>
        <Route path="/" element={<IMSTab />} />
      </Route>
    </Routes>,
    { auth: { role } },
  );

describe("Integrations IMS section", () => {
  it("shows the P-CSCF addresses", async () => {
    api.get(IMS_PATH, () => imsSettings(["10.6.0.5", "2001:db8::5"]));
    renderTab();

    expect(
      await (await row("P-CSCF Addresses")).findByText("10.6.0.5, 2001:db8::5"),
    ).toBeInTheDocument();
  });

  it("updates the P-CSCF addresses", async () => {
    const user = userEvent.setup();
    api.put(IMS_PATH, () => ({}));
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Edit P-CSCF addresses" }),
    );
    await user.type(
      await screen.findByLabelText(/^Addresses/),
      "10.6.0.5, 2001:db8::5",
    );
    await user.click(screen.getByRole("button", { name: /^Update$/ }));

    await screen.findByText("P-CSCF addresses updated successfully.");
    expect(
      api.requests(IMS_PATH).find((r) => r.method === "PUT")?.body,
    ).toEqual({
      pcscfAddresses: ["10.6.0.5", "2001:db8::5"],
      homeDomain: "",
      advertiseVoice: true,
      restrictPeers: true,
    });
  });

  it("rejects malformed and duplicate P-CSCF addresses", async () => {
    const user = userEvent.setup();
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Edit P-CSCF addresses" }),
    );
    const input = await screen.findByLabelText(/^Addresses/);

    await user.type(input, "10.6.0.300");
    await user.tab();
    expect(
      await screen.findByText(
        "10.6.0.300 is not an IPv4 or IPv6 address or a host name",
      ),
    ).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, "10.6.0.5, 10.6.0.5");
    await user.tab();
    expect(
      await screen.findByText("10.6.0.5 is listed more than once"),
    ).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, "pcscf.example.org, PCSCF.example.org");
    await user.tab();
    expect(
      await screen.findByText("PCSCF.example.org is listed more than once"),
    ).toBeInTheDocument();
  });

  it("hides the P-CSCF edit from a read-only user", async () => {
    renderTab("Read Only");

    expect(await screen.findByText("P-CSCF Addresses")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit P-CSCF addresses" }),
    ).not.toBeInTheDocument();
  });
});

const mcpttAS = {
  name: "mcptt",
  serverName: "sip:mcptt.ims.example.org",
  publicServiceIdentities: ["sip:!.*!@mcptt.ims.example.org"],
};

const renderIMS = (role = "Admin") => {
  renderTab(role);
};

describe("Integrations IMS application servers", () => {
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
      serverName: "sip:conf.ims.example.org",
      publicServiceIdentities: [],
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
      serverName: "sip:as.ims.example.org",
      publicServiceIdentities: [],
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
  initialFilterCriteria: [
    {
      priority: 1,
      applicationServerName: "mcptt",
      defaultHandling: "SESSION_TERMINATED",
    },
    {
      priority: 0,
      applicationServerName: "ip-sm-gw",
      defaultHandling: "SESSION_CONTINUED",
    },
    {
      priority: 2,
      applicationServerName: "mcptt",
      defaultHandling: "SESSION_CONTINUED",
    },
  ],
};

describe("Integrations IMS service profiles", () => {
  it("lists each service profile with its application servers", async () => {
    api.get(SP_PATH, () => ({
      items: [mcpttSP, { name: "empty", initialFilterCriteria: [] }],
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
    ).toEqual({ name: "volte", initialFilterCriteria: [] });
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

describe("Integrations IMS without services", () => {
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

describe("Integrations IMS home domain", () => {
  it("shows the default domain when none is configured", async () => {
    renderTab();

    const home = await row("Home Domain");
    expect(
      await home.findByText("ims.mnc001.mcc001.3gppnetwork.org"),
    ).toBeInTheDocument();
    expect(home.getByText("Default")).toBeInTheDocument();
    expect(home.queryByText(/Also accepted/)).not.toBeInTheDocument();
  });

  it("shows a configured domain and the default one as also accepted", async () => {
    api.get(IMS_PATH, () => imsSettings([], { homeDomain: "ims.example.com" }));
    renderTab();

    const home = await row("Home Domain");
    expect(await home.findByText("ims.example.com")).toBeInTheDocument();
    expect(
      home.getByText("Also accepted: ims.mnc001.mcc001.3gppnetwork.org"),
    ).toBeInTheDocument();
  });

  it("changes the domain after confirming its effects", async () => {
    const user = userEvent.setup();
    api.get(IMS_PATH, () => imsSettings(["10.6.0.5"]));
    api.put(IMS_PATH, () => ({}));
    renderTab();

    await screen.findByText("10.6.0.5");
    await user.click(
      await screen.findByRole("button", { name: "Edit home domain" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    const input = dialog.getByLabelText(/^Home Domain/);
    expect(input).toHaveAttribute(
      "placeholder",
      "ims.mnc001.mcc001.3gppnetwork.org",
    );
    expect(
      dialog.getByText(
        "SIP domain of your subscribers' public identities. Must match your IMS and the domain on ISIMs. Leave empty to use the default.",
      ),
    ).toBeInTheDocument();

    await user.type(input, "IMS.Example.com");
    await user.click(dialog.getByRole("button", { name: /^Update$/ }));

    expect(
      await dialog.findByText(/Rewrites the public service identities/),
    ).toBeInTheDocument();
    expect(dialog.getByText(/Deregisters registered IMS users/)).toBeVisible();
    expect(
      dialog.getByText("Your IMS must be configured with the same domain."),
    ).toBeVisible();
    expect(putBody()).toBeUndefined();

    await user.click(dialog.getByRole("button", { name: /^Confirm$/ }));

    expect(
      await screen.findByText("Home domain updated successfully."),
    ).toBeInTheDocument();
    expect(putBody()).toEqual({
      pcscfAddresses: ["10.6.0.5"],
      homeDomain: "ims.example.com",
      advertiseVoice: true,
      restrictPeers: true,
    });
  });

  it("rejects a domain that is not a host name", async () => {
    const user = userEvent.setup();
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Edit home domain" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    await user.type(dialog.getByLabelText(/^Home Domain/), "localhost");
    await user.tab();

    expect(
      await dialog.findByText(/Must be a domain name with at least two labels/),
    ).toBeInTheDocument();
    expect(dialog.getByRole("button", { name: /^Update$/ })).toBeDisabled();
  });

  it("keeps the home domain when updating P-CSCF addresses", async () => {
    const user = userEvent.setup();
    api.get(IMS_PATH, () => imsSettings([], { homeDomain: "ims.example.com" }));
    api.put(IMS_PATH, () => ({}));
    renderTab();

    await screen.findByText("ims.example.com");
    await user.click(
      await screen.findByRole("button", { name: "Edit P-CSCF addresses" }),
    );
    await user.type(await screen.findByLabelText(/^Addresses/), "10.6.0.5");
    await user.click(screen.getByRole("button", { name: /^Update$/ }));

    await screen.findByText("P-CSCF addresses updated successfully.");
    expect(putBody()).toEqual({
      pcscfAddresses: ["10.6.0.5"],
      homeDomain: "ims.example.com",
      advertiseVoice: true,
      restrictPeers: true,
    });
  });
});

const scscf = {
  name: "scscf",
  originHost: "scscf.ims.example.org",
  realm: "ims.example.org",
  addresses: ["10.0.0.0/24"],
  connections: [
    {
      state: "open",
      originHost: "scscf.ims.example.org",
      realm: "ims.example.org",
      since: "2026-11-08T12:00:00Z",
    },
  ],
};

const icscf = {
  name: "icscf",
  realm: "ims.example.org",
  addresses: ["10.6.0.0/24"],
  connections: [],
};

describe("Integrations IMS Diameter peers", () => {
  it("warns while any peer can connect", async () => {
    api.get(IMS_PATH, () => imsSettings([], { restrictPeers: false }));
    renderTab();

    expect(
      await screen.findByText(
        "Any system that can reach the Diameter port can query subscriber credentials. Turn on Restrict to Allowed Peers to block them.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("No allowed peers yet.")).toBeInTheDocument();
  });

  it("lists each peer with its connections", async () => {
    api.get(IMS_PEERS_PATH, () => ({ items: [scscf, icscf] }));
    renderTab();

    const s = await row("scscf");
    expect(s.getByText("scscf.ims.example.org")).toBeInTheDocument();
    expect(s.getByText("Connected")).toBeInTheDocument();

    const i = await row("icscf");
    expect(i.getByText("Any")).toBeInTheDocument();
    expect(i.getByText("10.6.0.0/24")).toBeInTheDocument();
    expect(i.getByText("Not connected")).toBeInTheDocument();
    expect(
      screen.queryByText(/Any system that can reach the Diameter port/),
    ).not.toBeInTheDocument();
  });

  it("adds a peer by realm and source address", async () => {
    const user = userEvent.setup();
    let peers: unknown[] = [];
    api.get(IMS_PEERS_PATH, () => ({ items: peers }));
    api.post(IMS_PEERS_PATH, () => {
      peers = [icscf];
      return {};
    });
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Add allowed peer" }),
    );
    expect(await screen.findByRole("dialog")).toHaveAccessibleName(
      "Add Allowed Peer",
    );
    const dialog = within(screen.getByRole("dialog"));
    const add = dialog.getByRole("button", { name: /^Add$/ });
    await user.type(dialog.getByLabelText(/^Name/), "icscf");
    await user.type(dialog.getByLabelText(/^Realm/), "ims.example.org");
    expect(add).toBeDisabled();

    await user.type(dialog.getByLabelText(/^Source Addresses/), "10.6.0.0/24");
    await waitFor(() => expect(add).toBeEnabled());
    await user.click(add);

    expect(
      await screen.findByText("Allowed peer added successfully."),
    ).toBeVisible();
    expect(
      api.requests(IMS_PEERS_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({
      name: "icscf",
      originHost: "",
      realm: "ims.example.org",
      addresses: ["10.6.0.0/24"],
    });
  });

  it("rejects an Origin-Host that is not a domain name", async () => {
    const user = userEvent.setup();
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Add allowed peer" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    await user.type(dialog.getByLabelText(/^Origin-Host/), "scscf");
    await user.tab();

    expect(
      await dialog.findByText(/Origin-Host must be a fully qualified domain/),
    ).toBeInTheDocument();
  });

  it("edits a peer without changing its name", async () => {
    const user = userEvent.setup();
    api.get(IMS_PEERS_PATH, () => ({ items: [scscf] }));
    api.put(`${IMS_PEERS_PATH}/scscf`, () => ({}));
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Edit allowed peer scscf" }),
    );
    expect(await screen.findByRole("dialog")).toHaveAccessibleName(
      "Edit Allowed Peer",
    );
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByLabelText(/^Name/)).toBeDisabled();
    await user.clear(dialog.getByLabelText(/^Realm/));
    const update = dialog.getByRole("button", { name: /^Update$/ });
    await waitFor(() => expect(update).toBeEnabled());
    await user.click(update);

    expect(
      await screen.findByText("Allowed peer updated successfully."),
    ).toBeVisible();
    expect(api.lastRequest(`${IMS_PEERS_PATH}/scscf`)?.body).toEqual({
      originHost: "scscf.ims.example.org",
      realm: "",
      addresses: ["10.0.0.0/24"],
    });
  });

  it("warns before deleting the last peer", async () => {
    const user = userEvent.setup();
    let peers: unknown[] = [scscf];
    api.get(IMS_PEERS_PATH, () => ({ items: peers }));
    api.delete(`${IMS_PEERS_PATH}/scscf`, () => {
      peers = [];
      return {};
    });
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Delete allowed peer scscf" }),
    );
    expect(await screen.findByText(/It is your last one/)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toHaveTextContent("Delete Allowed Peer");
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(
      await screen.findByText("No allowed peers yet."),
    ).toBeInTheDocument();
  });

  it("hides peer writes from a read-only user", async () => {
    api.get(IMS_PEERS_PATH, () => ({ items: [scscf] }));
    renderTab("Read Only");

    await row("scscf");
    expect(
      screen.queryByRole("button", { name: "Add allowed peer" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /allowed peer scscf/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit home domain" }),
    ).not.toBeInTheDocument();
  });
});

describe("Integrations IMS tab Diameter identity", () => {
  it("shows the values to enter in the IMS", async () => {
    renderTab();

    const table = within(
      await screen.findByRole("table", { name: "This core" }),
    );
    expect(
      await table.findByText("epc.mnc001.mcc001.3gppnetwork.org"),
    ).toBeInTheDocument();
    expect(
      table.getByText("mmec01.mmegi8001.mme.epc.mnc001.mcc001.3gppnetwork.org"),
    ).toBeInTheDocument();
    expect(table.getByText("10.0.0.2:3868 (SCTP)")).toBeInTheDocument();
    expect(
      table.getByRole("button", { name: "Copy Origin-Host" }),
    ).toBeInTheDocument();
  });

  it("lists the IMS settings after the Diameter identity", async () => {
    renderTab();

    const table = await screen.findByRole("table", { name: "This core" });
    await within(table).findByText("10.0.0.2:3868 (SCTP)");
    const rows = within(table).getAllByRole("row");
    expect(
      rows.map((r) => within(r).getAllByRole("cell")[0].textContent),
    ).toEqual([
      "Diameter Realm",
      "Origin-Host",
      "Diameter Address",
      "P-CSCF Addresses",
      "Home Domain",
      "Advertise IMS Voice",
      "Restrict to Allowed Peers",
    ]);
    expect(
      rows.map((r) => within(r).queryAllByRole("button", { name: /^Edit / })),
    ).toEqual([
      [],
      [],
      [],
      [within(rows[3]).getByRole("button", { name: "Edit P-CSCF addresses" })],
      [within(rows[4]).getByRole("button", { name: "Edit home domain" })],
      [
        within(rows[5]).getByRole("button", {
          name: "Edit IMS voice advertisement",
        }),
      ],
      [
        within(rows[6]).getByRole("button", {
          name: "Edit IMS peer restriction",
        }),
      ],
    ]);
  });

  it("labels each node's Origin-Host in a cluster", async () => {
    stubDiameterStatus(api, {
      ...diameterStatus,
      nodes: [
        { ...diameterStatus.nodes[0], displayName: "core-1" },
        {
          nodeId: "2",
          displayName: "core-2",
          originHost: "mmec02.mmegi8001.mme.epc.mnc001.mcc001.3gppnetwork.org",
          local: false,
        },
      ],
    });
    renderTab();

    expect(
      await screen.findByText("Origin-Host (this node)"),
    ).toBeInTheDocument();
    expect(screen.getByText("Origin-Host (core-2)")).toBeInTheDocument();
  });

  it("says when Diameter is not listening", async () => {
    stubDiameterStatus(api, {
      ...diameterStatus,
      listener: { enabled: false, addresses: [] },
    });
    renderTab();

    expect(await screen.findByText(/Not listening/)).toBeInTheDocument();
  });

  it("lists unknown peers and adds one by its Origin-Host", async () => {
    const user = userEvent.setup();
    stubDiameterStatus(api, {
      ...diameterStatus,
      peers: [
        {
          id: "rogue",
          role: "unknown",
          originHost: "rogue.other.example.org",
          realm: "other.example.org",
          address: "10.0.0.66",
          direction: "inbound",
          state: "open",
          since: "2026-11-08T12:00:00Z",
        },
      ],
    });
    api.post(IMS_PEERS_PATH, () => ({}));
    renderTab();

    expect(await screen.findByText("Unknown Peers (1)")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", {
        name: "Add rogue.other.example.org as allowed peer",
      }),
    );

    const dialog = within(await screen.findByRole("dialog"));
    expect(dialog.getByLabelText(/^Name/)).toHaveValue("rogue");
    expect(dialog.getByLabelText(/^Realm/)).toHaveValue("");
    expect(dialog.getByLabelText(/^Source Addresses/)).toHaveValue(
      "10.0.0.66/32",
    );
    const add = dialog.getByRole("button", { name: /^Add$/ });
    await waitFor(() => expect(add).toBeEnabled());
    await user.click(add);

    await waitFor(() =>
      expect(
        api.requests(IMS_PEERS_PATH).find((r) => r.method === "POST")?.body,
      ).toEqual({
        name: "rogue",
        originHost: "rogue.other.example.org",
        realm: "",
        addresses: ["10.0.0.66/32"],
      }),
    );
  });

  it("hides unknown peers when there are none", async () => {
    renderTab();

    await screen.findByRole("table", { name: "This core" });
    expect(screen.queryByText(/Unknown Peers/)).not.toBeInTheDocument();
  });

  it("puts the status right before the actions", async () => {
    renderTab();

    const headers = within(
      await screen.findByRole("table", { name: "Allowed IMS peers" }),
    )
      .getAllByRole("cell")
      .slice(0, 6)
      .map((c) => c.textContent);
    expect(headers).toEqual([
      "Name",
      "Origin-Host",
      "Realm",
      "Source Addresses",
      "Status",
      "Actions",
    ]);
  });
});

describe("Integrations IMS allowed peer source addresses", () => {
  it("adds a peer by source addresses, as CIDRs", async () => {
    const user = userEvent.setup();
    api.post(IMS_PEERS_PATH, () => ({}));
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Add allowed peer" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    await user.type(dialog.getByLabelText(/^Name/), "pcscf");
    await user.type(
      dialog.getByLabelText(/^Source Addresses/),
      "10.0.0.5, 10.1.0.0/16, 2001:db8::5",
    );
    const add = dialog.getByRole("button", { name: /^Add$/ });
    await waitFor(() => expect(add).toBeEnabled());
    await user.click(add);

    await screen.findByText("Allowed peer added successfully.");
    expect(
      api.requests(IMS_PEERS_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({
      name: "pcscf",
      originHost: "",
      realm: "",
      addresses: ["10.0.0.5/32", "10.1.0.0/16", "2001:db8::5/128"],
    });
  });

  it("rejects source addresses that are not IPs or CIDRs", async () => {
    const user = userEvent.setup();
    renderTab();

    await user.click(
      await screen.findByRole("button", { name: "Add allowed peer" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    const input = dialog.getByLabelText(/^Source Addresses/);

    await user.click(input);
    await user.tab();
    expect(
      await dialog.findByText("At least one source address is required"),
    ).toBeInTheDocument();

    await user.type(input, "10.0.0.0/33");
    await user.tab();
    expect(
      await dialog.findByText(
        "10.0.0.0/33 is not an IP address or CIDR, for example 10.0.0.0/24",
      ),
    ).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, "pcscf.example.org");
    await user.tab();
    expect(
      await dialog.findByText(/^pcscf\.example\.org is not an IP address/),
    ).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, "10.0.0.5, 10.0.0.5/32");
    await user.tab();
    expect(
      await dialog.findByText("10.0.0.5/32 is listed more than once"),
    ).toBeInTheDocument();
  });

  it("shows each peer's source addresses", async () => {
    api.get(IMS_PEERS_PATH, () => ({
      items: [{ ...scscf, addresses: ["10.0.0.0/24", "2001:db8::/64"] }],
    }));
    renderTab();

    expect(
      (await row("scscf")).getByText("10.0.0.0/24, 2001:db8::/64"),
    ).toBeInTheDocument();
  });
});

describe("Integrations IMS This Core", () => {
  it("lists the other realms Ella Core answers for", async () => {
    stubDiameterStatus(api, {
      ...diameterStatus,
      servedRealms: [
        diameterStatus.realm,
        "ims.mnc001.mcc001.3gppnetwork.org",
        "ims.example.com",
      ],
    });
    renderTab();

    const realm = await row("Diameter Realm");
    expect(
      await realm.findByText(
        "Also accepts: ims.mnc001.mcc001.3gppnetwork.org, ims.example.com",
      ),
    ).toBeInTheDocument();
  });

  it("omits the line when only the Diameter realm is served", async () => {
    stubDiameterStatus(api, {
      ...diameterStatus,
      servedRealms: [diameterStatus.realm],
    });
    renderTab();

    await (await row("Diameter Realm")).findByText(diameterStatus.realm);
    expect(screen.queryByText(/Also accepts/)).not.toBeInTheDocument();
  });

  it("says when the Origin-Host is not assigned yet", async () => {
    stubDiameterStatus(api, { ...diameterStatus, nodes: [] });
    renderTab();

    expect(
      await screen.findByText("Assigned when this node joins the cluster."),
    ).toBeInTheDocument();
  });

  it("names the service profiles' application servers", async () => {
    renderTab();

    expect(
      await screen.findByText("No service profiles yet."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("No service profiles yet.").closest("tbody")!,
    ).toHaveTextContent("Application Servers");
    expect(screen.queryByText("Filter Criteria")).not.toBeInTheDocument();
  });
});

describe("Integrations IMS voice readiness", () => {
  beforeEach(() => {
    api.get(IMS_PEERS_PATH, () => ({ items: [icscf] }));
  });

  it.each([
    ["no_pcscf", /no P-CSCF address is set/],
    ["no_ims_data_network", /there is no IMS data network named ims/],
    ["ip_family_mismatch", /no P-CSCF address matches the ims data network/],
  ])("warns that calls fail when %s", async (voiceStatus, cause) => {
    api.get(IMS_PATH, () => imsSettings([], { voiceStatus }));
    renderTab();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      /Phones are told IMS voice is available, but calls fail/,
    );
    expect(alert).toHaveTextContent(cause);
  });

  it("links to the data networks when there is no IMS data network", async () => {
    api.get(IMS_PATH, () =>
      imsSettings([], { voiceStatus: "no_ims_data_network" }),
    );
    renderTab();

    expect(
      within(await screen.findByRole("alert")).getByRole("link", {
        name: "Networking › Data Networks",
      }),
    ).toHaveAttribute("href", "/networking/data-networks");
  });

  it("says phones are not told when voice is ready but not advertised", async () => {
    api.get(IMS_PATH, () =>
      imsSettings(["10.6.0.5"], {
        voiceStatus: "enabled",
        advertiseVoice: false,
      }),
    );
    renderTab();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "IMS voice is ready, but phones are not told it is available: Advertise IMS Voice is off.",
    );
  });

  it("shows no alert when voice is ready and advertised", async () => {
    api.get(IMS_PATH, () =>
      imsSettings(["10.6.0.5"], { voiceStatus: "enabled" }),
    );
    api.get(IMS_PEERS_PATH, () => ({ items: [icscf] }));
    renderTab();

    await (await row("P-CSCF Addresses")).findByText("10.6.0.5");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not warn about calls when voice is not advertised", async () => {
    api.get(IMS_PATH, () => imsSettings([], { advertiseVoice: false }));
    api.get(IMS_PEERS_PATH, () => ({ items: [icscf] }));
    renderTab();

    expect(
      await (await row("Advertise IMS Voice")).findByText("Off"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("Integrations IMS voice advertisement", () => {
  it("shows whether voice is advertised", async () => {
    renderTab();

    expect(
      await (await row("Advertise IMS Voice")).findByText("On"),
    ).toBeInTheDocument();
  });

  it("turns the advertisement off and keeps the other settings", async () => {
    const user = userEvent.setup();
    api.get(IMS_PATH, () =>
      imsSettings(["10.6.0.5"], { homeDomain: "ims.example.com" }),
    );
    api.put(IMS_PATH, () => ({}));
    renderTab();

    await user.click(
      await (
        await row("Advertise IMS Voice")
      ).findByRole("button", { name: "Edit IMS voice advertisement" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    const toggle = dialog.getByRole("switch", { name: "Advertise IMS voice" });
    expect(toggle).toBeChecked();
    await user.click(toggle);
    await user.click(dialog.getByRole("button", { name: "Update" }));

    await screen.findByText("IMS voice advertisement updated successfully.");
    expect(putBody()).toEqual({
      pcscfAddresses: ["10.6.0.5"],
      homeDomain: "ims.example.com",
      advertiseVoice: false,
      restrictPeers: true,
    });
  });

  it("hides the edit from a read-only user", async () => {
    renderTab("Read Only");

    await (await row("Advertise IMS Voice")).findByText("On");
    expect(
      screen.queryByRole("button", { name: "Edit IMS voice advertisement" }),
    ).not.toBeInTheDocument();
  });
});

describe("Integrations IMS peer restriction", () => {
  it("shows whether peers are restricted", async () => {
    renderTab();

    expect(
      await (await row("Restrict to Allowed Peers")).findByText("On"),
    ).toBeInTheDocument();
  });

  it("warns that every IMS node is rejected when no peer is allowed", async () => {
    renderTab();

    expect(
      await screen.findByText("No allowed peers: every IMS node is rejected."),
    ).toBeInTheDocument();
  });

  it("does not warn when restricted to a listed peer", async () => {
    api.get(IMS_PEERS_PATH, () => ({ items: [icscf] }));
    renderTab();

    await row("icscf");
    expect(
      screen.queryByText("No allowed peers: every IMS node is rejected."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Any system that can reach the Diameter port/),
    ).not.toBeInTheDocument();
  });

  it("warns while unrestricted even with allowed peers", async () => {
    api.get(IMS_PATH, () => imsSettings([], { restrictPeers: false }));
    api.get(IMS_PEERS_PATH, () => ({ items: [icscf] }));
    renderTab();

    expect(
      await screen.findByText(/Any system that can reach the Diameter port/),
    ).toBeInTheDocument();
  });

  it("turns the restriction on and keeps the other settings", async () => {
    const user = userEvent.setup();
    api.get(IMS_PATH, () =>
      imsSettings(["10.6.0.5"], {
        homeDomain: "ims.example.com",
        restrictPeers: false,
      }),
    );
    api.put(IMS_PATH, () => ({}));
    renderTab();

    await user.click(
      await (
        await row("Restrict to Allowed Peers")
      ).findByRole("button", { name: "Edit IMS peer restriction" }),
    );
    const dialog = within(await screen.findByRole("dialog"));
    const toggle = dialog.getByRole("switch", {
      name: "Restrict to allowed peers",
    });
    expect(toggle).not.toBeChecked();
    await user.click(toggle);
    await user.click(dialog.getByRole("button", { name: "Update" }));

    await screen.findByText("IMS peer restriction updated successfully.");
    expect(putBody()).toEqual({
      pcscfAddresses: ["10.6.0.5"],
      homeDomain: "ims.example.com",
      advertiseVoice: true,
      restrictPeers: true,
    });
  });

  it("hides the edit from a read-only user", async () => {
    renderTab("Read Only");

    await (await row("Restrict to Allowed Peers")).findByText("On");
    expect(
      screen.queryByRole("button", { name: "Edit IMS peer restriction" }),
    ).not.toBeInTheDocument();
  });
});
