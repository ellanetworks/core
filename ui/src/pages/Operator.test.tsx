// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import { PRODUCT } from "@/utils/product";
import Operator from "./Operator";

const api = setupApiServer();

const SMSCS_PATH = "/api/v1/operator/sms/smscs";
const AS_PATH = "/api/v1/operator/ims/application-servers";
const SP_PATH = "/api/v1/operator/ims/service-profiles";
const PARTNERS_PATH = "/api/v1/operator/roaming/partners";
const DIAMETER_PEERS_PATH = "/api/v1/operator/roaming/diameter-peers";
const SEPP_PATH = "/api/v1/operator/roaming/sepp";
const seppCertPath = (seppID: number | string) =>
  `${SEPP_PATH}/nodes/${seppID}/certificate`;

beforeEach(() => {
  api.get(AS_PATH, () => ({ items: [] }));
  api.get(SP_PATH, () => ({ items: [] }));
  api.get(PARTNERS_PATH, () => ({ items: [] }));
  api.get(DIAMETER_PEERS_PATH, () => ({ items: [] }));
  api.get(SEPP_PATH, () => ({
    listener: { configured: false, listening: false },
    trustAnchors: [],
    certificates: [],
  }));
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

const nodeA = {
  diameterIdentity: "smsc-a.example.org",
  address: "2001:db8::10",
  port: 3868,
  status: {
    state: "open",
    host: "smsc-a.example.org",
    realm: "example.org",
    since: "2026-09-29T16:00:00Z",
  },
};

const nodeB = {
  diameterIdentity: "smsc-b.example.org",
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

const nodeStatus = async (endpoint: string) =>
  (await screen.findByText(endpoint)).nextElementSibling as HTMLElement;

describe("Operator SMS section", () => {
  it("shows an unset SMS number and no SMSCs", async () => {
    api.get("/api/v1/operator", () => operator({ smsNumber: "" }));
    api.get(SMSCS_PATH, () => ({ items: [] }));
    renderOperator();

    expect(
      await (await row("SMS Number")).findByText("N/A"),
    ).toBeInTheDocument();
    expect(screen.getByText("SMSCs")).toBeInTheDocument();
    expect(await screen.findByText("No SMSCs yet.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Edit SMS number" }),
    ).toBeInTheDocument();
  });

  it("lists each SMSC with the status of its nodes", async () => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({
      items: [main, backup],
    }));
    renderOperator();

    expect(await screen.findByText("+15550001111")).toBeInTheDocument();

    const a = await row("main");
    expect(a.getByText("+15550000000")).toBeInTheDocument();
    expect(await nodeStatus("[2001:db8::10]:3868")).toHaveTextContent(
      "Connected",
    );

    const b = await row("backup");
    expect(b.getByText("+15550000001, +15550000002")).toBeInTheDocument();
    expect(await nodeStatus("[2001:db8::11]:3868")).toHaveTextContent(
      "Connected",
    );
    expect(await nodeStatus("192.0.2.20:3869")).toHaveTextContent(
      "Disconnected",
    );
  });

  it("shows why a node is disconnected", async () => {
    const user = userEvent.setup();
    const error =
      "peer answered as smsc.example.org, expected smsc-b.example.org";
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({
      items: [
        { ...main, nodes: [{ ...nodeB, status: { ...nodeB.status, error } }] },
      ],
    }));
    renderOperator();

    await user.hover(await nodeStatus("192.0.2.20:3869"));

    expect(await screen.findByRole("tooltip")).toHaveTextContent(error);
  });

  it("shows a node this node has not reported as connecting", async () => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({
      items: [{ ...main, nodes: [{ ...nodeA, status: undefined }] }],
    }));
    renderOperator();

    expect(await nodeStatus("[2001:db8::10]:3868")).toHaveTextContent(
      "Connecting",
    );
  });

  it("adds an SMSC", async () => {
    const user = userEvent.setup();
    let smscs: unknown[] = [];
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({ items: smscs }));
    api.post(SMSCS_PATH, () => {
      smscs = [{ ...main, nodes: [nodeB] }];
      return {};
    });
    renderOperator();

    await user.click(await screen.findByRole("button", { name: "Add SMSC" }));
    await user.type(await screen.findByLabelText(/^Name/), "main");
    await user.type(screen.getByLabelText(/^Numbers/), "+15550000000");
    await user.type(screen.getByLabelText(/^Node 1 address/), "192.0.2.20");
    await user.type(
      screen.getByLabelText(/^Node 1 Diameter identity/),
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
          diameterIdentity: "smsc-b.example.org",
          address: "192.0.2.20",
          port: 3868,
        },
      ],
    });
  });

  it("warns before deleting the only SMSC", async () => {
    const user = userEvent.setup();
    let smscs: unknown[] = [main];
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({ items: smscs }));
    api.delete(`${SMSCS_PATH}/main`, () => {
      smscs = [];
      return {};
    });
    renderOperator();

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
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({ items: [main] }));
    renderOperator("Read Only");

    expect(await screen.findByText("[2001:db8::10]:3868")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit SMS number" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add SMSC" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Delete SMSC/ }),
    ).not.toBeInTheDocument();
  });
});

describe("Operator IMS section", () => {
  it("shows the P-CSCF addresses", async () => {
    api.get("/api/v1/operator", () =>
      operator(ready, ["10.6.0.5", "2001:db8::5"]),
    );
    api.get(SMSCS_PATH, () => ({ items: [] }));
    renderOperator();

    expect(
      await (await row("P-CSCF Addresses")).findByText("10.6.0.5, 2001:db8::5"),
    ).toBeInTheDocument();
  });

  it("updates the P-CSCF addresses", async () => {
    const user = userEvent.setup();
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({ items: [] }));
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
    api.get(SMSCS_PATH, () => ({ items: [] }));
    renderOperator();

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
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({ items: [] }));
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
  api.get(SMSCS_PATH, () => ({ items: [] }));
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
    {
      priority: 2,
      application_server_name: "mcptt",
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

const PEM =
  "-----BEGIN CERTIFICATE-----\nMIIBszCCAVmgAwIBAgIUQ==\n-----END CERTIFICATE-----";

const HOME_DOMAIN = "5gc.mnc001.mcc001.3gppnetwork.org";

const hss = {
  id: "0190a000-0000-7000-8000-0000000000d1",
  diameterIdentity: "hss.epc.mnc002.mcc208.3gppnetwork.org",
  address: "192.0.2.30",
  port: 3868,
};

const dea = {
  id: "0190a000-0000-7000-8000-0000000000d2",
  diameterIdentity: "",
  address: "2001:db8::30",
  port: 3869,
};

const orange = {
  id: "0190a000-0000-7000-8000-0000000000a1",
  name: "Orange",
  plmns: [
    { mcc: "208", mnc: "01" },
    { mcc: "208", mnc: "02" },
  ],
  profileName: "roamers",
  sepp: {
    fqdn: "sepp1.5gc.mnc001.mcc208.3gppnetwork.org",
    n32cURI: "https://sepp.orange.example:443",
    trustAnchorPEM: PEM,
  },
  n32: {
    state: "established",
  },
};

const sfr = {
  id: "0190a000-0000-7000-8000-0000000000a2",
  name: "SFR",
  plmns: [{ mcc: "208", mnc: "10" }],
  profileName: "default",
  diameterPeerID: hss.id,
};

const days = (n: number) =>
  new Date(Date.now() + n * 24 * 60 * 60 * 1000).toISOString();

const summary = (notAfter = days(1000)) => ({
  subject: "CN=sepp",
  sans: [],
  issuer: "CN=Ella Core SEPP CA",
  notBefore: days(-1),
  notAfter,
  fingerprint: "sha256:ab12cd34",
  trustAnchorFingerprint: "sha256:ca11",
});

const certificate = (seppID: number) => ({
  seppID,
  fqdn: `sepp${seppID}.${HOME_DOMAIN}`,
  source: "generated",
  ...summary(),
});

const seppStatus = (notAfter = days(1000), fqdn1 = `sepp1.${HOME_DOMAIN}`) => ({
  listener: {
    seppID: 1,
    configured: true,
    address: "192.0.2.5",
    port: 443,
    listening: true,
  },
  trustAnchors: [
    {
      source: "generated",
      subject: "CN=Ella Core SEPP CA",
      fingerprint: "sha256:ca11",
      notAfter: days(3000),
      pem: `${PEM}\n`,
      seppIDs: [1, 9],
    },
    {
      source: "imported",
      subject: "CN=Operator Root CA",
      fingerprint: "sha256:ca22",
      notAfter: days(3000),
      pem: `${PEM}\n`,
      seppIDs: [2],
    },
  ],
  certificates: [
    {
      seppID: 1,
      fqdn: fqdn1,
      source: "generated",
      ...summary(notAfter),
    },
    {
      seppID: 2,
      fqdn: `sepp2.${HOME_DOMAIN}`,
      source: "imported",
      ...summary(),
      trustAnchorFingerprint: "sha256:ca22",
    },
    {
      seppID: 9,
      fqdn: `sepp9.${HOME_DOMAIN}`,
      source: "generated",
      ...summary(),
    },
  ],
});

const dialog = () => screen.getByRole("dialog");

const sent = (path: string, method: string) =>
  api
    .requests(path)
    .filter((r) => r.method === method)
    .at(-1)?.body;

const setText = async (
  user: ReturnType<typeof userEvent.setup>,
  label: RegExp,
  value: string,
) => {
  const input = within(dialog()).getByRole("textbox", { name: label });
  await user.clear(input);
  if (value) fireEvent.change(input, { target: { value } });
  await user.tab();
};

const table = async (name: string) =>
  within(await screen.findByRole("table", { name }));

const tableRow = async (name: string, text: string) => {
  const cell = await (await table(name)).findByText(text);
  return within(cell.closest("tr")!);
};

describe("Operator roaming section", () => {
  beforeEach(() => {
    api.get("/api/v1/operator", () => operator(ready));
    api.get(SMSCS_PATH, () => ({ items: [] }));
    api.get(PARTNERS_PATH, () => ({ items: [orange, sfr] }));
    api.get(DIAMETER_PEERS_PATH, () => ({ items: [hss, dea] }));
    api.get("/api/v1/profiles", () => ({
      items: [{ name: "default" }, { name: "roamers" }],
      page: 1,
      per_page: 100,
      total_count: 2,
    }));
    api.get(SEPP_PATH, () => seppStatus());
  });

  describe("partners", () => {
    it("lists partners with their PLMNs, profile, N32 status and Diameter peer", async () => {
      renderOperator();

      const orangeRow = await tableRow("Roaming partners", "Orange");
      expect(orangeRow.getByText("208-01, 208-02")).toBeInTheDocument();
      expect(orangeRow.getByRole("link", { name: "roamers" })).toHaveAttribute(
        "href",
        "/profiles/roamers",
      );
      expect(orangeRow.getByText("Established")).toBeInTheDocument();

      const sfrRow = await tableRow("Roaming partners", "SFR");
      expect(
        await sfrRow.findByText(
          "hss.epc.mnc002.mcc208.3gppnetwork.org (192.0.2.30:3868)",
        ),
      ).toBeInTheDocument();
      expect(sfrRow.queryByText("Established")).not.toBeInTheDocument();
    });

    it("shows why the N32 connection with a partner failed", async () => {
      const user = userEvent.setup();
      api.get(PARTNERS_PATH, () => ({
        items: [
          {
            ...orange,
            n32: {
              state: "failed",
              lastError: "the partner SEPP refused the exchange-capability",
            },
          },
        ],
      }));
      renderOperator();

      await user.hover(
        (await tableRow("Roaming partners", "Orange")).getByText("Failed"),
      );
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "the partner SEPP refused the exchange-capability",
      );
    });

    it("restarts the N32 handshake with a 5G partner only", async () => {
      const user = userEvent.setup();
      api.post(`${PARTNERS_PATH}/:id/handshake`, () => ({
        n32: { state: "establishing" },
      }));
      renderOperator();

      await user.click(
        await screen.findByRole("button", {
          name: "Restart N32 handshake with Orange",
        }),
      );

      expect(
        await screen.findByText("N32 handshake with Orange started."),
      ).toBeInTheDocument();
      expect(
        api
          .requests(`${PARTNERS_PATH}/${orange.id}/handshake`)
          .filter((r) => r.method === "POST"),
      ).toHaveLength(1);
      expect(
        screen.queryByRole("button", {
          name: "Restart N32 handshake with SFR",
        }),
      ).not.toBeInTheDocument();
    });

    it("shows the handshake refusal", async () => {
      const user = userEvent.setup();
      api.post(`${PARTNERS_PATH}/:id/handshake`, () =>
        httpError(503, "The SEPP leader is not available, retry shortly"),
      );
      renderOperator();

      await user.click(
        await screen.findByRole("button", {
          name: "Restart N32 handshake with Orange",
        }),
      );

      expect(
        await screen.findByText(
          /Failed to start the N32 handshake: The SEPP leader is not available/,
        ),
      ).toBeInTheDocument();
    });

    it("confirms deleting a partner and shows the refusal", async () => {
      const user = userEvent.setup();
      api.delete(`${PARTNERS_PATH}/:id`, () =>
        httpError(409, "roaming partner Orange has 2 roamers"),
      );
      renderOperator();

      await user.click(
        await screen.findByRole("button", {
          name: "Delete roaming partner Orange",
        }),
      );
      expect(
        within(dialog()).getByText(
          "Delete the roaming partner Orange? Its roamers will be deregistered.",
        ),
      ).toBeInTheDocument();
      await user.click(
        within(dialog()).getByRole("button", { name: "Confirm" }),
      );

      await waitFor(() =>
        expect(api.lastRequest(`${PARTNERS_PATH}/${orange.id}`)?.method).toBe(
          "DELETE",
        ),
      );
      expect(
        await screen.findByText(/roaming partner Orange has 2 roamers/),
      ).toBeInTheDocument();
    });

    it("adds a partner with 5G and 4G roaming", async () => {
      const user = userEvent.setup();
      api.post(PARTNERS_PATH, () => ({ ...orange, id: "new" }));
      renderOperator();

      await user.click(
        await screen.findByRole("button", { name: "Add Partner" }),
      );
      await setText(user, /^Name/, "Vodafone");
      await setText(user, /^MCC 1/, "262");
      await setText(user, /^MNC 1/, "02");
      await user.click(
        within(dialog()).getByRole("button", { name: "Add PLMN" }),
      );
      await setText(user, /^MCC 2/, "262");
      await setText(user, /^MNC 2/, "09");

      await user.click(
        within(dialog()).getByRole("combobox", { name: "Profile" }),
      );
      await user.click(await screen.findByRole("option", { name: "roamers" }));

      await user.click(
        within(dialog()).getByRole("switch", { name: "5G Roaming" }),
      );
      await setText(
        user,
        /^SEPP FQDN/,
        "sepp1.5gc.mnc009.mcc262.3gppnetwork.org",
      );
      expect(
        within(dialog()).getByLabelText(/^NRF API Root 2/),
      ).toHaveAttribute(
        "placeholder",
        "https://nrf.5gc.mnc009.mcc262.3gppnetwork.org",
      );
      await setText(user, /^NRF API Root 1/, "http://nrf.vodafone.example");
      await setText(user, /^N32-c URI/, "https://sepp.vodafone.example");
      await user.upload(
        within(dialog()).getByLabelText("Trust Anchor file"),
        new File([PEM], "ca.pem", { type: "application/x-pem-file" }),
      );
      await waitFor(() =>
        expect(within(dialog()).getByLabelText(/^Trust Anchor$/)).toHaveValue(
          PEM,
        ),
      );

      await user.click(
        within(dialog()).getByRole("switch", { name: "4G Roaming" }),
      );
      await user.click(
        within(dialog()).getByRole("combobox", { name: "Diameter Peer" }),
      );
      await user.click(
        await screen.findByRole("option", { name: "[2001:db8::30]:3869" }),
      );

      const create = within(dialog()).getByRole("button", { name: "Create" });
      await waitFor(() => expect(create).toBeEnabled());
      await user.click(create);

      await waitFor(() =>
        expect(sent(PARTNERS_PATH, "POST")).toEqual({
          name: "Vodafone",
          plmns: [
            {
              mcc: "262",
              mnc: "02",
              nrfAPIRoot: "http://nrf.vodafone.example",
            },
            { mcc: "262", mnc: "09" },
          ],
          profileName: "roamers",
          sepp: {
            fqdn: "sepp1.5gc.mnc009.mcc262.3gppnetwork.org",
            n32cURI: "https://sepp.vodafone.example",
            trustAnchorPEM: `${PEM}\n`,
          },
          diameterPeerID: dea.id,
        }),
      );
      expect(
        await screen.findByText("Roaming partner added successfully."),
      ).toBeInTheDocument();
    });

    it("rejects overlapping PLMNs and a SEPP FQDN outside them", async () => {
      const user = userEvent.setup();
      renderOperator();

      await user.click(
        await screen.findByRole("button", { name: "Add Partner" }),
      );
      await setText(user, /^MCC 1/, "262");
      await setText(user, /^MNC 1/, "02");
      await user.click(
        within(dialog()).getByRole("button", { name: "Add PLMN" }),
      );
      await setText(user, /^MCC 2/, "262");
      await setText(user, /^MNC 2/, "002");
      await user.click(
        within(dialog()).getByRole("switch", { name: "5G Roaming" }),
      );
      await setText(
        user,
        /^SEPP FQDN/,
        "sepp1.5gc.mnc001.mcc262.3gppnetwork.org",
      );
      await setText(user, /^N32-c URI/, "http://sepp.example.org");
      await setText(user, /^NRF API Root 1/, "ftp://nrf.example.org");

      expect(
        await within(dialog()).findByText(/NRF API root must be an http/),
      ).toBeInTheDocument();
      expect(
        await within(dialog()).findByText(
          "SEPP FQDN must name one of the partner's PLMNs",
        ),
      ).toBeInTheDocument();
      expect(
        await within(dialog()).findByText(/N32-c URI must be an https URI/),
      ).toBeInTheDocument();
      expect(
        await within(dialog()).findByText("PLMNs must not overlap"),
      ).toBeInTheDocument();
      expect(
        within(dialog()).getByRole("button", { name: "Create" }),
      ).toBeDisabled();
    });

    it("edits the PLMNs and drops 5G when switched off", async () => {
      const user = userEvent.setup();
      api.put(`${PARTNERS_PATH}/:id`, () => undefined);
      renderOperator();

      await user.click(
        await screen.findByRole("button", {
          name: "Edit roaming partner Orange",
        }),
      );
      expect(within(dialog()).getByLabelText(/^SEPP FQDN/)).toHaveValue(
        orange.sepp.fqdn,
      );
      await user.click(
        within(dialog()).getByRole("button", { name: "Remove PLMN 2" }),
      );
      await user.click(
        within(dialog()).getByRole("switch", { name: "5G Roaming" }),
      );
      await user.click(
        within(dialog()).getByRole("button", { name: "Update" }),
      );

      await waitFor(() =>
        expect(sent(`${PARTNERS_PATH}/${orange.id}`, "PUT")).toEqual({
          name: "Orange",
          plmns: [{ mcc: "208", mnc: "01" }],
          profileName: "roamers",
        }),
      );
    });
  });

  describe("Diameter peers", () => {
    it("lists peers with the partners using them and blocks deleting one in use", async () => {
      renderOperator();

      const used = await tableRow("Diameter peers", "192.0.2.30:3868");
      expect(used.getByText("SFR")).toBeInTheDocument();
      expect(
        used.getByRole("button", {
          name: "Delete Diameter peer 192.0.2.30:3868",
        }),
      ).toBeDisabled();

      const unused = await tableRow("Diameter peers", "[2001:db8::30]:3869");
      expect(unused.getByText("Any")).toBeInTheDocument();
      expect(
        unused.getByRole("button", {
          name: "Delete Diameter peer [2001:db8::30]:3869",
        }),
      ).toBeEnabled();
    });

    it("adds a peer", async () => {
      const user = userEvent.setup();
      api.post(DIAMETER_PEERS_PATH, () => ({ ...hss, id: "new" }));
      renderOperator();

      await user.click(
        await screen.findByRole("button", { name: "Add Diameter Peer" }),
      );
      await setText(user, /^Address/, "198.51.100.7");
      await setText(user, /^Diameter Identity/, "dea.example.org");
      const create = within(dialog()).getByRole("button", { name: "Create" });
      await waitFor(() => expect(create).toBeEnabled());
      await user.click(create);

      await waitFor(() =>
        expect(sent(DIAMETER_PEERS_PATH, "POST")).toEqual({
          diameterIdentity: "dea.example.org",
          address: "198.51.100.7",
          port: 3868,
        }),
      );
    });

    it("deletes an unused peer", async () => {
      const user = userEvent.setup();
      api.delete(`${DIAMETER_PEERS_PATH}/:id`, () => undefined);
      renderOperator();

      await user.click(
        await screen.findByRole("button", {
          name: "Delete Diameter peer [2001:db8::30]:3869",
        }),
      );
      await user.click(
        within(dialog()).getByRole("button", { name: "Confirm" }),
      );

      await waitFor(() =>
        expect(
          api.lastRequest(`${DIAMETER_PEERS_PATH}/${dea.id}`)?.method,
        ).toBe("DELETE"),
      );
    });
  });

  describe("SEPP", () => {
    it("shows this node's SEPP certificate and trust anchor", async () => {
      renderOperator("Read Only");

      const cert = await tableRow("SEPP", "Certificate");
      expect(
        cert.getByText((text) =>
          text.startsWith(`sepp1.${HOME_DOMAIN} · Generated · Expires `),
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText("Listening")).not.toBeInTheDocument();
      expect(screen.queryByText(/192\.0\.2\.5/)).not.toBeInTheDocument();
      expect(cert.queryByText(/^Expire/)).not.toBeInTheDocument();

      const anchor = await tableRow("SEPP", "Trust anchor");
      expect(anchor.getByText("Give to your partners")).toBeInTheDocument();
      expect(
        anchor.getByRole("button", {
          name: "Download trust anchor sha256:ca11",
        }),
      ).toBeEnabled();
      expect(
        anchor.getByRole("button", {
          name: "Download trust anchor sha256:ca22",
        }),
      ).toBeEnabled();

      expect((await table("SEPP")).getAllByRole("row")).toHaveLength(2);
      expect(
        screen.queryByRole("table", { name: "Trust anchors" }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(`sepp2.${HOME_DOMAIN}`),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("sha256:ca11")).not.toBeInTheDocument();
    });

    it("shows the SEPP as disabled when this node has no N32 interface", async () => {
      api.get(SEPP_PATH, () => ({
        ...seppStatus(),
        listener: { configured: false, listening: false },
      }));
      renderOperator();

      const sepp = await tableRow("SEPP", "SEPP");
      expect(sepp.getByText(/Disabled on this node/)).toBeInTheDocument();
      expect(sepp.getByRole("link", { name: /Learn more/ })).toHaveAttribute(
        "href",
        PRODUCT.roamingDocsUrl,
      );
      const rows = (await table("SEPP")).getAllByRole("row");
      expect(rows).toHaveLength(1);
      expect(
        screen.queryByRole("button", { name: /Import certificate/ }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /Regenerate/ }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /Download trust anchor/ }),
      ).not.toBeInTheDocument();
    });

    it("warns about an expiring certificate", async () => {
      api.get(SEPP_PATH, () => seppStatus(days(10.5)));
      renderOperator();

      expect(
        (await tableRow("SEPP", "Certificate")).getByText("Expires in 10 days"),
      ).toBeInTheDocument();
    });

    it("warns about an expired certificate", async () => {
      api.get(SEPP_PATH, () => seppStatus(days(-2)));
      renderOperator();

      expect(
        (await tableRow("SEPP", "Certificate")).getByText("Expired"),
      ).toBeInTheDocument();
    });

    it("flags this node's SEPP FQDN outside the operator PLMN", async () => {
      api.get(SEPP_PATH, () =>
        seppStatus(days(1000), "sepp1.5gc.mnc001.mcc208.3gppnetwork.org"),
      );
      renderOperator();

      expect(
        await screen.findByText(
          `The SEPP FQDN of this node is not under ${HOME_DOMAIN}. Import a new certificate.`,
        ),
      ).toBeInTheDocument();
    });

    it("ignores other nodes' SEPP FQDNs outside the operator PLMN", async () => {
      api.get(SEPP_PATH, () => ({
        ...seppStatus(),
        certificates: [
          ...seppStatus().certificates,
          {
            seppID: 4,
            fqdn: "sepp4.5gc.mnc001.mcc208.3gppnetwork.org",
            source: "imported",
            ...summary(),
          },
        ],
      }));
      renderOperator();

      await tableRow("SEPP", "Certificate");
      expect(screen.queryByText(/is not under/)).not.toBeInTheDocument();
    });

    it("downloads the trust anchor", async () => {
      const user = userEvent.setup();
      const click = vi.fn();
      window.URL.createObjectURL = vi.fn(() => "blob:anchor");
      window.URL.revokeObjectURL = vi.fn();
      const realCreate = document.createElement.bind(document);
      let link: HTMLAnchorElement | undefined;
      const spy = vi
        .spyOn(document, "createElement")
        .mockImplementation((tag: string) => {
          const el = realCreate(tag);
          if (tag === "a") {
            el.click = click;
            link = el as HTMLAnchorElement;
          }
          return el;
        });
      api.get(SEPP_PATH, () => ({
        ...seppStatus(),
        trustAnchors: seppStatus().trustAnchors.slice(0, 1),
      }));
      renderOperator("Read Only");

      const button = (await tableRow("SEPP", "Trust anchor")).getByRole(
        "button",
        { name: "Download trust anchor" },
      );
      await user.hover(button);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "sha256:ca11",
      );
      await user.click(button);

      expect(click).toHaveBeenCalled();
      expect(link?.download).toBe("sepp-trust-anchor-ca11.pem");
      spy.mockRestore();
    });

    it("imports a certificate for this node and shows the API warnings", async () => {
      const user = userEvent.setup();
      api.put(seppCertPath(":seppID"), () => ({
        ...certificate(1),
        source: "imported",
        warnings: ["The certificate is valid for more than 3 years"],
      }));
      renderOperator();

      await user.click(
        (await tableRow("SEPP", "Certificate")).getByRole("button", {
          name: "Import certificate",
        }),
      );
      expect(within(dialog()).getByLabelText(/^FQDN/)).toHaveAttribute(
        "placeholder",
        `sepp1.${HOME_DOMAIN}`,
      );
      await setText(user, /^Certificate Chain/, PEM);
      await setText(
        user,
        /^Private Key/,
        "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
      );
      const importButton = within(dialog()).getByRole("button", {
        name: "Import",
      });
      await waitFor(() => expect(importButton).toBeEnabled());
      await user.click(importButton);

      expect(
        await screen.findByText(
          "The certificate is valid for more than 3 years",
        ),
      ).toBeInTheDocument();
      expect(sent(seppCertPath(1), "PUT")).toEqual({
        certificateChainPEM: PEM,
        privateKeyPEM:
          "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
      });
    });

    it("regenerates this node's certificate by deleting it", async () => {
      const user = userEvent.setup();
      api.delete(seppCertPath(":seppID"), () => ({ message: "deleted" }));
      renderOperator();

      await user.click(
        (await tableRow("SEPP", "Certificate")).getByRole("button", {
          name: "Regenerate certificate",
        }),
      );
      await user.click(
        within(dialog()).getByRole("button", { name: "Regenerate" }),
      );

      expect(
        await screen.findByText("SEPP certificate regenerated successfully."),
      ).toBeInTheDocument();
      expect(api.lastRequest(seppCertPath(1))?.method).toBe("DELETE");
      expect(
        screen.queryByText(/trust anchor of this node changed/),
      ).not.toBeInTheDocument();
    });

    it("warns that the trust anchor changes when an imported certificate is replaced", async () => {
      const user = userEvent.setup();
      api.get(SEPP_PATH, () => ({
        ...seppStatus(),
        certificates: [{ ...seppStatus().certificates[0], source: "imported" }],
      }));
      api.delete(seppCertPath(":seppID"), () => ({ message: "deleted" }));
      renderOperator();

      await user.click(
        (await tableRow("SEPP", "Certificate")).getByRole("button", {
          name: "Regenerate certificate",
        }),
      );
      await user.click(
        within(dialog()).getByRole("button", { name: "Regenerate" }),
      );

      expect(
        await screen.findByText(/The trust anchor of this node changed/),
      ).toBeInTheDocument();
    });

    it("regenerates the trust anchor and shows the API warnings", async () => {
      const user = userEvent.setup();
      api.post(`${SEPP_PATH}/ca/regenerate`, () => ({
        trustAnchor: seppStatus().trustAnchors[0],
        warnings: ["Every roaming partner must import the new trust anchor"],
      }));
      renderOperator();

      await user.click(
        (await tableRow("SEPP", "Trust anchor")).getByRole("button", {
          name: "Regenerate trust anchor",
        }),
      );
      expect(
        within(dialog()).getByText(
          "Partners must import the new trust anchor before roaming works again.",
        ),
      ).toBeInTheDocument();
      await user.click(
        within(dialog()).getByRole("button", { name: "Regenerate" }),
      );

      expect(
        await screen.findByText(
          "Every roaming partner must import the new trust anchor",
        ),
      ).toBeInTheDocument();
      expect(
        await screen.findByText("SEPP trust anchor regenerated successfully."),
      ).toBeInTheDocument();
      expect(api.lastRequest(`${SEPP_PATH}/ca/regenerate`)?.method).toBe(
        "POST",
      );
    });

    it("cancels a regeneration without calling the API", async () => {
      const user = userEvent.setup();
      renderOperator();

      for (const [rowName, name] of [
        ["Certificate", "Regenerate certificate"],
        ["Trust anchor", "Regenerate trust anchor"],
      ]) {
        await user.click(
          (await tableRow("SEPP", rowName)).getByRole("button", { name }),
        );
        await user.click(
          within(dialog()).getByRole("button", { name: "Cancel" }),
        );
        await waitFor(() =>
          expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
        );
      }

      expect(api.requests(seppCertPath(1))).toHaveLength(0);
      expect(api.requests(`${SEPP_PATH}/ca/regenerate`)).toHaveLength(0);
    });
  });

  it("hides roaming writes from a read-only user", async () => {
    renderOperator("Read Only");

    await tableRow("Roaming partners", "Orange");
    await tableRow("SEPP", "Certificate");
    for (const name of [
      "Add Partner",
      "Add Diameter Peer",
      /Edit roaming partner/,
      /Delete roaming partner/,
      /Restart N32 handshake/,
      /Edit Diameter peer/,
      /Delete Diameter peer/,
      /Import certificate/,
      /Regenerate certificate/,
      /Regenerate trust anchor/,
    ]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
    expect(
      screen.getByRole("button", {
        name: "Download trust anchor sha256:ca22",
      }),
    ).toBeEnabled();
  });
});
