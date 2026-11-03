// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navigate, Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import Roaming from "./Roaming";
import PartnersTab from "./roaming/PartnersTab";
import DiameterPeersTab from "./roaming/DiameterPeersTab";
import UsageTab from "./roaming/UsageTab";
import SEPPTab from "./roaming/SEPPTab";

const api = setupApiServer();

const PARTNERS_PATH = "/api/v1/roaming/partners";
const PEERS_PATH = "/api/v1/roaming/diameter-peers";
const SEPP_STATUS_PATH = "/api/v1/roaming/sepp";
const SEPP_CA_PATH = `${SEPP_STATUS_PATH}/ca/regenerate`;
const seppCertPath = (seppID: number | string) =>
  `${SEPP_STATUS_PATH}/nodes/${seppID}/certificate`;
const USAGE_PATH = "/api/v1/roaming/usage";

const PEM =
  "-----BEGIN CERTIFICATE-----\nMIIBszCCAVmgAwIBAgIUQ==\n-----END CERTIFICATE-----";

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
  n32: [
    {
      nodeID: "node-a",
      seppID: 1,
      state: "established",
      since: "2026-11-03T10:00:00Z",
      initiator: "peer",
      nftlst: true,
    },
    {
      nodeID: "node-b",
      seppID: 2,
      state: "unknown",
      nftlst: false,
      lastError: "dial timeout",
    },
  ],
};

const sfr = {
  id: "0190a000-0000-7000-8000-0000000000a2",
  name: "SFR",
  plmns: [{ mcc: "208", mnc: "10" }],
  profileName: "default",
  diameterPeerID: hss.id,
};

const operator = {
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
  spn: { fullName: "Ella", shortName: "Ella" },
  sms: { smsNumber: "" },
  ims: { pcscfAddresses: [], voiceStatus: "no_pcscf" },
};

const days = (n: number) =>
  new Date(Date.now() + n * 24 * 60 * 60 * 1000).toISOString();

const HOME_DOMAIN = "5gc.mnc001.mcc001.3gppnetwork.org";

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
  certificateChainPEM: `${PEM}\n${PEM}\n`,
  trustAnchorPEM: `${PEM}\n`,
  fingerprint: "sha256:ab12cd34",
  notAfter: days(1000),
});

const seppStatus = (notAfter = days(1000), fqdn1 = `sepp1.${HOME_DOMAIN}`) => ({
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
  nodes: [
    {
      nodeID: "node-a",
      seppID: 1,
      fqdn: fqdn1,
      source: "generated",
      reachable: true,
      n32Configured: true,
      address: "192.0.2.5",
      port: 443,
      listening: true,
      certificate: summary(notAfter),
    },
    {
      nodeID: "node-b",
      seppID: 2,
      fqdn: `sepp2.${HOME_DOMAIN}`,
      source: "imported",
      reachable: true,
      n32Configured: false,
      listening: false,
      certificate: { ...summary(), trustAnchorFingerprint: "sha256:ca22" },
    },
    {
      nodeID: "node-c",
      seppID: 3,
      reachable: false,
      n32Configured: false,
      listening: false,
      error: "unknown: dial timeout",
    },
    {
      seppID: 9,
      fqdn: `sepp9.${HOME_DOMAIN}`,
      source: "generated",
      reachable: false,
      n32Configured: false,
      listening: false,
      certificate: summary(),
      error: "no cluster node has this SEPP ID",
    },
  ],
});

beforeEach(() => {
  api.get(PARTNERS_PATH, () => ({ items: [orange, sfr] }));
  api.get(PEERS_PATH, () => ({ items: [hss, dea] }));
  api.get("/api/v1/profiles", () => ({
    items: [{ name: "default" }, { name: "roamers" }],
    page: 1,
    per_page: 100,
    total_count: 2,
  }));
  api.get(SEPP_STATUS_PATH, () => seppStatus());
  api.get(seppCertPath(":seppID"), (req) =>
    certificate(Number(req.url.pathname.split("/").at(-2))),
  );
  api.get("/api/v1/operator", () => operator);
});

const renderRoaming = (path: string, role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route path="/roaming" element={<Roaming />}>
        <Route index element={<Navigate to="partners" replace />} />
        <Route path="partners" element={<PartnersTab />} />
        <Route path="diameter-peers" element={<DiameterPeersTab />} />
        <Route path="usage" element={<UsageTab />} />
        <Route path="sepp" element={<SEPPTab />} />
      </Route>
    </Routes>,
    { initialEntries: [path], auth: { role } },
  );

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

describe("Roaming partners", () => {
  it("lists partners with their 5G and 4G settings", async () => {
    renderRoaming("/roaming/partners");

    expect(await screen.findByText("Orange")).toBeInTheDocument();
    expect(screen.getByText("208-01, 208-02")).toBeInTheDocument();
    expect(screen.getByText("roamers")).toBeInTheDocument();
    expect(
      screen.getByText("sepp1.5gc.mnc001.mcc208.3gppnetwork.org"),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "hss.epc.mnc002.mcc208.3gppnetwork.org (192.0.2.30:3868)",
      ),
    ).toBeInTheDocument();
  });

  it("shows the live N32 status of partners with a 5G agreement", async () => {
    const user = userEvent.setup();
    api.get(PARTNERS_PATH, () => ({
      items: [
        orange,
        sfr,
        {
          ...orange,
          id: "0190a000-0000-7000-8000-0000000000a3",
          name: "Bouygues",
          plmns: [{ mcc: "208", mnc: "20" }],
          sepp: {
            ...orange.sepp,
            fqdn: "sepp1.5gc.mnc020.mcc208.3gppnetwork.org",
          },
          n32: [
            {
              nodeID: "node-a",
              seppID: 1,
              state: "failed",
              since: "2026-11-03T10:00:00Z",
              nftlst: false,
              lastError: "the partner SEPP refused the exchange-capability",
            },
          ],
        },
      ],
    }));
    renderRoaming("/roaming/partners");

    expect(await screen.findByText("SEPP 1: Established")).toBeInTheDocument();
    expect(screen.getByText("SEPP 2: Unknown")).toBeInTheDocument();
    const failed = screen.getByText("SEPP 1: Failed");
    await user.hover(failed);
    expect(
      await screen.findByText(
        "the partner SEPP refused the exchange-capability",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/^SEPP \d+: /)).toHaveLength(3);
  });

  it("re-runs the N32 handshake with a partner", async () => {
    const user = userEvent.setup();
    api.post(`${PARTNERS_PATH}/:id/handshake`, () => ({
      n32: [
        {
          nodeID: "node-a",
          seppID: 1,
          state: "establishing",
          since: "2026-11-03T10:00:00Z",
          nftlst: false,
        },
        {
          nodeID: "node-b",
          seppID: 2,
          state: "unknown",
          nftlst: false,
          lastError: "node unreachable",
        },
      ],
    }));
    renderRoaming("/roaming/partners");

    await user.click(
      await screen.findByRole("button", {
        name: "Re-run N32 handshake with Orange",
      }),
    );

    await waitFor(() =>
      expect(
        api
          .requests(`${PARTNERS_PATH}/${orange.id}/handshake`)
          .filter((r) => r.method === "POST"),
      ).toHaveLength(1),
    );
    expect(
      await screen.findByText(
        'N32-c handshake with "Orange" started: SEPP 1 establishing, SEPP 2 unknown (node unreachable).',
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Re-run N32 handshake with SFR",
      }),
    ).not.toBeInTheDocument();
  });

  it("shows the handshake refusal", async () => {
    const user = userEvent.setup();
    api.post(`${PARTNERS_PATH}/:id/handshake`, () =>
      httpError(503, "The SEPP leader is not available, retry shortly"),
    );
    renderRoaming("/roaming/partners");

    await user.click(
      await screen.findByRole("button", {
        name: "Re-run N32 handshake with Orange",
      }),
    );

    expect(
      await screen.findByText(
        /Failed to start the N32-c handshake: The SEPP leader is not available/,
      ),
    ).toBeInTheDocument();
  });

  it("hides write actions from read-only users", async () => {
    renderRoaming("/roaming/partners", "Read Only");

    await screen.findByText("Orange");
    expect(
      screen.queryByRole("button", { name: "Add Partner" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /Delete roaming partner/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Delete roaming partner/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Edit roaming partner/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Re-run N32 handshake/ }),
    ).not.toBeInTheDocument();
  });

  it("lets read-only users view a partner's full configuration", async () => {
    const user = userEvent.setup();
    renderRoaming("/roaming/partners", "Read Only");

    await user.click(
      await screen.findByRole("button", {
        name: "View roaming partner Orange",
      }),
    );

    const view = dialog();
    expect(within(view).getByText("208-01, 208-02")).toBeInTheDocument();
    expect(
      within(view).getByText("https://sepp.orange.example:443"),
    ).toBeInTheDocument();
    expect(within(view).getByText("Same as N32-c")).toBeInTheDocument();
    expect(
      within(view).getByText(
        "Default (https://nrf.5gc.mnc001.mcc208.3gppnetwork.org)",
      ),
    ).toBeInTheDocument();
    expect(within(view).getByLabelText("Trust anchor")).toHaveTextContent(
      "BEGIN CERTIFICATE",
    );
    const n32 = within(view).getByLabelText("N32-c status");
    expect(within(n32).getByText("SEPP 1 (node-a)")).toBeInTheDocument();
    expect(within(n32).getByText("Established")).toBeInTheDocument();
    expect(within(n32).getByText("SEPP 2 (node-b)")).toBeInTheDocument();
    expect(within(n32).getByText("Unknown")).toBeInTheDocument();
    expect(within(n32).getByText("dial timeout")).toBeInTheDocument();
    expect(
      within(view).getByText("No 4G roaming agreement."),
    ).toBeInTheDocument();
    expect(
      within(view).queryByRole("button", { name: "Update" }),
    ).not.toBeInTheDocument();
  });

  it("confirms deleting a partner and shows a refusal while it has roamers", async () => {
    const user = userEvent.setup();
    api.delete(`${PARTNERS_PATH}/:id`, () =>
      httpError(409, "roaming partner Orange has 2 roamers"),
    );
    renderRoaming("/roaming/partners");

    await user.click(
      await screen.findByRole("button", {
        name: "Delete roaming partner Orange",
      }),
    );

    expect(
      within(dialog()).getByText(
        'Delete the roaming partner "Orange" (PLMN 208-01, 208-02)? Its roamers are detached first and purged at their home network. Its roaming usage is kept. This cannot be undone.',
      ),
    ).toBeInTheDocument();

    await user.click(within(dialog()).getByRole("button", { name: "Confirm" }));

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
    renderRoaming("/roaming/partners");

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
      within(dialog()).getByRole("switch", { name: "5G roaming" }),
    );
    expect(within(dialog()).getByLabelText(/^NRF API Root/)).toHaveAttribute(
      "placeholder",
      "https://nrf.5gc.mnc002.mcc262.3gppnetwork.org",
    );
    await setText(
      user,
      /^SEPP FQDN/,
      "sepp1.5gc.mnc009.mcc262.3gppnetwork.org",
    );
    expect(within(dialog()).getByLabelText(/^NRF API Root/)).toHaveAttribute(
      "placeholder",
      "https://nrf.5gc.mnc009.mcc262.3gppnetwork.org",
    );
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
      within(dialog()).getByRole("switch", { name: "4G roaming" }),
    );
    await user.click(
      within(dialog()).getByRole("combobox", { name: "Diameter Peer" }),
    );
    await user.click(
      await screen.findByRole("option", { name: "[2001:db8::30]:3869" }),
    );

    const add = within(dialog()).getByRole("button", { name: "Add" });
    await waitFor(() => expect(add).toBeEnabled());
    await user.click(add);

    await waitFor(() =>
      expect(sent(PARTNERS_PATH, "POST")).toEqual({
        name: "Vodafone",
        plmns: [
          { mcc: "262", mnc: "02" },
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
  });

  it("rejects an N32-c URI that is not https", async () => {
    const user = userEvent.setup();
    renderRoaming("/roaming/partners");

    await user.click(
      await screen.findByRole("button", { name: "Add Partner" }),
    );
    await user.click(
      within(dialog()).getByRole("switch", { name: "5G roaming" }),
    );
    await setText(user, /^N32-c URI/, "http://sepp.example.org");

    expect(
      await within(dialog()).findByText(/N32-c URI must be an https URI/),
    ).toBeInTheDocument();
  });

  it("rejects duplicate PLMNs and a SEPP FQDN outside the partner's PLMNs", async () => {
    const user = userEvent.setup();
    renderRoaming("/roaming/partners");

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
      within(dialog()).getByRole("switch", { name: "5G roaming" }),
    );
    await setText(
      user,
      /^SEPP FQDN/,
      "sepp1.5gc.mnc001.mcc262.3gppnetwork.org",
    );

    expect(
      await within(dialog()).findByText(
        "SEPP FQDN must name one of the partner's PLMNs",
      ),
    ).toBeInTheDocument();

    await setText(user, /^SEPP FQDN/, "sepp.5gc.mnc002.mcc262.3gppnetwork.org");
    expect(
      await within(dialog()).findByText(/must not be the well-known/),
    ).toBeInTheDocument();

    expect(
      within(dialog()).getByRole("button", { name: "Add" }),
    ).toBeDisabled();
    expect(
      await within(dialog()).findByText(/Two PLMNs are the same or ambiguous/),
    ).toBeInTheDocument();
  });

  it("edits the PLMNs and drops 5G when switched off", async () => {
    const user = userEvent.setup();
    api.put(`${PARTNERS_PATH}/:id`, () => undefined);
    renderRoaming("/roaming/partners");

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
    expect(
      within(dialog()).getByRole("button", { name: "Remove PLMN 1" }),
    ).toBeDisabled();
    await user.click(
      within(dialog()).getByRole("button", { name: "Add PLMN" }),
    );
    await setText(user, /^MCC 2/, "208");
    await setText(user, /^MNC 2/, "05");

    await user.click(
      within(dialog()).getByRole("switch", { name: "5G roaming" }),
    );
    await user.click(within(dialog()).getByRole("button", { name: "Update" }));

    await waitFor(() =>
      expect(sent(`${PARTNERS_PATH}/${orange.id}`, "PUT")).toEqual({
        name: "Orange",
        plmns: [
          { mcc: "208", mnc: "01" },
          { mcc: "208", mnc: "05" },
        ],
        profileName: "roamers",
      }),
    );
  });

  it("shows the API error when the partner is refused", async () => {
    const user = userEvent.setup();
    api.put(`${PARTNERS_PATH}/:id`, () =>
      httpError(409, "PLMN 208-10 has roamers and cannot be removed"),
    );
    renderRoaming("/roaming/partners");

    await user.click(
      await screen.findByRole("button", { name: "Edit roaming partner SFR" }),
    );
    await user.click(within(dialog()).getByRole("button", { name: "Update" }));

    expect(
      await within(dialog()).findByText(
        "Failed to update roaming partner: PLMN 208-10 has roamers and cannot be removed",
      ),
    ).toBeInTheDocument();
  });
});

describe("Roaming Diameter peers", () => {
  it("lists peers with the partners that use them and blocks deleting one in use", async () => {
    renderRoaming("/roaming/diameter-peers");

    expect(await screen.findByText("192.0.2.30:3868")).toBeInTheDocument();
    expect(screen.getByText("Any")).toBeInTheDocument();
    expect(await screen.findByText("SFR")).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Diameter peer 192.0.2.30:3868 is used by a partner",
      }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", {
        name: "Delete Diameter peer [2001:db8::30]:3869",
      }),
    ).toBeEnabled();
  });

  it("adds a peer", async () => {
    const user = userEvent.setup();
    api.post(PEERS_PATH, () => ({ ...hss, id: "new" }));
    renderRoaming("/roaming/diameter-peers");

    await user.click(await screen.findByRole("button", { name: "Add Peer" }));
    await setText(user, /^Address/, "198.51.100.7");
    await setText(user, /^Diameter Identity/, "dea.example.org");

    const add = within(dialog()).getByRole("button", { name: "Add" });
    await waitFor(() => expect(add).toBeEnabled());
    await user.click(add);

    await waitFor(() =>
      expect(sent(PEERS_PATH, "POST")).toEqual({
        diameterIdentity: "dea.example.org",
        address: "198.51.100.7",
        port: 3868,
      }),
    );
  });
});

describe("Roaming SEPP", () => {
  const nodesTable = () => screen.findByRole("table", { name: "SEPP nodes" });
  const nodeRow = async (seppID: string) => {
    const table = await nodesTable();
    const cell = await within(table).findByRole("cell", { name: seppID });
    return cell.closest("tr") as HTMLElement;
  };

  it("lists the trust anchors and the SEPPs using them", async () => {
    renderRoaming("/roaming/sepp", "Read Only");

    const table = await screen.findByRole("table", { name: "Trust anchors" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText("Cluster SEPP CA")).toBeInTheDocument();
    expect(
      within(rows[0]).getByText("CN=Ella Core SEPP CA"),
    ).toBeInTheDocument();
    expect(within(rows[0]).getByText("sha256:ca11")).toBeInTheDocument();
    expect(within(rows[0]).getByText("SEPP 1, SEPP 9")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Imported CA")).toBeInTheDocument();
    expect(within(rows[1]).getByText("SEPP 2")).toBeInTheDocument();
    expect(
      within(rows[1]).getByRole("button", {
        name: "Download trust anchor CN=Operator Root CA",
      }),
    ).toBeEnabled();
    expect(
      within(rows[1]).getByRole("button", {
        name: "Copy trust anchor CN=Operator Root CA",
      }),
    ).toBeEnabled();
  });

  it("shows the SEPP identity, certificate and N32 listener of every node", async () => {
    const user = userEvent.setup();
    renderRoaming("/roaming/sepp", "Read Only");

    const a = await nodeRow("1");
    expect(within(a).getByText("node-a")).toBeInTheDocument();
    expect(within(a).getByText(`sepp1.${HOME_DOMAIN}`)).toBeInTheDocument();
    expect(within(a).getByText("Generated")).toBeInTheDocument();
    expect(within(a).getByText("192.0.2.5:443")).toBeInTheDocument();
    expect(within(a).getByText("Listening")).toBeInTheDocument();
    expect(within(a).queryByText(/Expires in/)).not.toBeInTheDocument();

    const b = await nodeRow("2");
    expect(within(b).getByText("Imported")).toBeInTheDocument();
    expect(within(b).getByText("Not configured")).toBeInTheDocument();

    const c = await nodeRow("3");
    expect(within(c).getByText("unknown: dial timeout")).toBeInTheDocument();
    await user.hover(within(c).getByText("Unknown"));
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "unknown: dial timeout",
    );

    const orphan = await nodeRow("9");
    expect(within(orphan).getByText("No node")).toBeInTheDocument();
    expect(
      within(orphan).getByText("no cluster node has this SEPP ID"),
    ).toBeInTheDocument();
  });

  it("warns when a node certificate expires soon", async () => {
    api.get(SEPP_STATUS_PATH, () => seppStatus(days(10.5)));
    renderRoaming("/roaming/sepp");

    const a = await nodeRow("1");
    expect(within(a).getByText("Expires in 10 days")).toBeInTheDocument();
  });

  it("flags an expired node certificate", async () => {
    api.get(SEPP_STATUS_PATH, () => seppStatus(days(-2)));
    renderRoaming("/roaming/sepp");

    const a = await nodeRow("1");
    expect(within(a).getByText("Expired")).toBeInTheDocument();
    expect(
      screen.getByText(/The certificate of SEPP 1 expired on/),
    ).toBeInTheDocument();
  });

  it("flags a node FQDN outside the operator PLMN", async () => {
    api.get(SEPP_STATUS_PATH, () =>
      seppStatus(days(1000), "sepp1.5gc.mnc001.mcc208.3gppnetwork.org"),
    );
    renderRoaming("/roaming/sepp");

    expect(
      await screen.findByText(
        /The FQDN of SEPP 1, sepp1\.5gc\.mnc001\.mcc208\.3gppnetwork\.org, is not under 5gc\.mnc001\.mcc001\.3gppnetwork\.org/,
      ),
    ).toBeInTheDocument();
  });

  it("downloads a node certificate chain", async () => {
    const user = userEvent.setup();
    const click = vi.fn();
    window.URL.createObjectURL = vi.fn(() => "blob:chain");
    window.URL.revokeObjectURL = vi.fn();
    const realCreate = document.createElement.bind(document);
    const spy = vi
      .spyOn(document, "createElement")
      .mockImplementation((tag: string) => {
        const el = realCreate(tag);
        if (tag === "a") el.click = click;
        return el;
      });
    renderRoaming("/roaming/sepp", "Read Only");

    await user.click(
      within(await nodeRow("2")).getByRole("button", {
        name: "Download certificate chain of SEPP 2",
      }),
    );

    await waitFor(() => expect(click).toHaveBeenCalled());
    expect(api.lastRequest(seppCertPath(2))?.method).toBe("GET");
    spy.mockRestore();
  });

  it("imports a node certificate and shows the API warnings", async () => {
    const user = userEvent.setup();
    api.put(seppCertPath(":seppID"), () => ({
      ...certificate(2),
      source: "imported",
      warnings: ["The certificate is valid for more than 3 years"],
    }));
    renderRoaming("/roaming/sepp");

    await user.click(
      within(await nodeRow("2")).getByRole("button", {
        name: "Import certificate for SEPP 2",
      }),
    );
    expect(within(dialog()).getByLabelText(/^FQDN/)).toHaveAttribute(
      "placeholder",
      `sepp2.${HOME_DOMAIN}`,
    );
    await setText(user, /^Certificate Chain/, PEM);
    await setText(
      user,
      /^Private Key/,
      "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
    );
    await setText(user, /^FQDN/, `sepp2.${HOME_DOMAIN}`);
    const importButton = within(dialog()).getByRole("button", {
      name: "Import",
    });
    await waitFor(() => expect(importButton).toBeEnabled());
    await user.click(importButton);

    expect(
      await screen.findByText("The certificate is valid for more than 3 years"),
    ).toBeInTheDocument();
    expect(sent(seppCertPath(2), "PUT")).toEqual({
      certificateChainPEM: PEM,
      privateKeyPEM:
        "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
      fqdn: `sepp2.${HOME_DOMAIN}`,
    });
  });

  it("asks for confirmation and generates a node certificate", async () => {
    const user = userEvent.setup();
    api.post(`${seppCertPath(":seppID")}/generate`, () => ({
      ...certificate(2),
      warnings: ["The trust anchor changed"],
    }));
    renderRoaming("/roaming/sepp");

    await user.click(
      within(await nodeRow("2")).getByRole("button", {
        name: "Generate certificate for SEPP 2",
      }),
    );
    expect(
      within(dialog()).getByText(
        /for sepp2\.5gc\.mnc001\.mcc001\.3gppnetwork\.org, signed by the cluster SEPP CA/,
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog()).getByText(/The current certificate is imported/),
    ).toBeInTheDocument();
    await user.click(
      within(dialog()).getByRole("button", { name: "Generate" }),
    );

    expect(
      await screen.findByText("The trust anchor changed"),
    ).toBeInTheDocument();
    expect(
      api
        .requests(`${seppCertPath(2)}/generate`)
        .filter((r) => r.method === "POST"),
    ).toHaveLength(1);
    expect(
      await screen.findByText("SEPP 2 certificate generated successfully."),
    ).toBeInTheDocument();
  });

  it("asks for confirmation and regenerates the SEPP CA", async () => {
    const user = userEvent.setup();
    api.post(SEPP_CA_PATH, () => ({
      trustAnchor: seppStatus().trustAnchors[0],
      reissued: [1, 9],
      warnings: ["Every partner must import the new trust anchor"],
    }));
    renderRoaming("/roaming/sepp");

    await user.click(
      await screen.findByRole("button", { name: "Regenerate CA" }),
    );
    expect(
      within(dialog()).getByText(
        /every roaming partner must import the new one/,
      ),
    ).toBeInTheDocument();
    await user.click(
      within(dialog()).getByRole("button", { name: "Regenerate" }),
    );

    expect(
      await screen.findByText(
        "SEPP CA regenerated. Reissued the certificates of SEPP 1, SEPP 9.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Every partner must import the new trust anchor"),
    ).toBeInTheDocument();
    expect(
      api.requests(SEPP_CA_PATH).filter((r) => r.method === "POST"),
    ).toHaveLength(1);
  });

  it("offers no import or generation for a SEPP ID without a node", async () => {
    renderRoaming("/roaming/sepp");

    const orphan = await nodeRow("9");
    expect(
      within(orphan).getByRole("button", {
        name: "Download certificate chain of SEPP 9",
      }),
    ).toBeEnabled();
    expect(
      within(orphan).queryByRole("button", { name: /Import certificate/ }),
    ).not.toBeInTheDocument();
    expect(
      within(orphan).queryByRole("button", { name: /Generate certificate/ }),
    ).not.toBeInTheDocument();
  });

  it("hides write actions from read-only users", async () => {
    renderRoaming("/roaming/sepp", "Read Only");

    await nodeRow("1");
    expect(
      screen.queryByRole("button", { name: "Regenerate CA" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Import certificate/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Generate certificate/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Download certificate chain of SEPP 1",
      }),
    ).toBeEnabled();
  });
});

describe("Roaming usage", () => {
  it("shows usage per PLMN and day with partner names and filters by IMSI", async () => {
    const user = userEvent.setup();
    api.get(USAGE_PATH, () => [
      {
        date: "2026-11-02",
        mcc: "208",
        mnc: "01",
        partner_name: "Orange",
        uplink_bytes: 1000,
        downlink_bytes: 4000,
        total_bytes: 5000,
      },
      {
        date: "2026-11-02",
        mcc: "310",
        mnc: "410",
        uplink_bytes: 10,
        downlink_bytes: 20,
        total_bytes: 30,
      },
    ]);
    renderRoaming("/roaming/usage");

    expect(await screen.findByText("Orange")).toBeInTheDocument();
    expect(screen.getByText("208-01")).toBeInTheDocument();
    expect(screen.getAllByText("310-410")).toHaveLength(2);
    expect(screen.getAllByText("2026-11-02").length).toBeGreaterThan(0);

    await user.type(screen.getByLabelText("IMSI"), "208010000000001");

    await waitFor(() =>
      expect(api.lastRequest(USAGE_PATH)?.params.get("imsi")).toBe(
        "208010000000001",
      ),
    );
    expect(api.lastRequest(USAGE_PATH)?.params.get("start")).toBeTruthy();
  });

  it("does not query with an invalid IMSI", async () => {
    const user = userEvent.setup();
    api.get(USAGE_PATH, () => []);
    renderRoaming("/roaming/usage");

    await screen.findByText("No roaming usage");
    await user.type(screen.getByLabelText("IMSI"), "12ab");

    expect(
      await screen.findByText("IMSI must be 6 to 15 digits"),
    ).toBeInTheDocument();
    expect(api.requests(USAGE_PATH).some((r) => r.params.has("imsi"))).toBe(
      false,
    );
  });

  it("hides the IMSI filter from roles without subscriber usage access", async () => {
    api.get(USAGE_PATH, () => []);
    renderRoaming("/roaming/usage", "Network Manager");

    await screen.findByText("No roaming usage");
    expect(screen.queryByLabelText("IMSI")).not.toBeInTheDocument();
  });
});
