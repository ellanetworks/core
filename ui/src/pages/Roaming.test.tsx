// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
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
const SEPP_PATH = "/api/v1/roaming/sepp/certificate";
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

const certificate = (notAfter = days(1000)) => ({
  fqdn: "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
  certificateChainPEM: `${PEM}\n${PEM}\n`,
  trustAnchorPEM: `${PEM}\n`,
  fingerprint: "sha256:ab12cd34",
  notAfter,
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
  api.get(SEPP_PATH, () => certificate());
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
        'Delete the roaming partner "Orange" (PLMN 208-01, 208-02)? A partner with registered roamers cannot be deleted. Its roaming usage is kept. This cannot be undone.',
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
  it("shows the certificate", async () => {
    renderRoaming("/roaming/sepp");

    expect(
      await screen.findByText("sepp1.5gc.mnc001.mcc001.3gppnetwork.org"),
    ).toBeInTheDocument();
    expect(screen.getByText("sha256:ab12cd34")).toBeInTheDocument();
    expect(screen.queryByText(/Expires in/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Regenerate" })).toBeEnabled();
  });

  it("shows the certificate's profile warnings", async () => {
    api.get(SEPP_PATH, () => ({
      ...certificate(),
      warnings: ["The certificate has no CRL distribution point"],
    }));
    renderRoaming("/roaming/sepp", "Read Only");

    expect(
      await screen.findByText("The certificate has no CRL distribution point"),
    ).toBeInTheDocument();
  });

  it("warns when the certificate expires soon", async () => {
    api.get(SEPP_PATH, () => certificate(days(10.5)));
    renderRoaming("/roaming/sepp");

    expect(await screen.findByText("Expires in 10 days")).toBeInTheDocument();
  });

  it("flags an expired certificate", async () => {
    api.get(SEPP_PATH, () => certificate(days(-2)));
    renderRoaming("/roaming/sepp");

    expect(await screen.findByText("Expired")).toBeInTheDocument();
    expect(
      screen.getByText(/The SEPP certificate expired on/),
    ).toBeInTheDocument();
  });

  it("shows an empty state and generates the first certificate", async () => {
    const user = userEvent.setup();
    api.get(SEPP_PATH, () => httpError(404, "SEPP certificate not found"));
    api.post(`${SEPP_PATH}/generate`, () => ({
      ...certificate(),
      warnings: ["The trust anchor changed"],
    }));
    renderRoaming("/roaming/sepp");

    expect(
      await screen.findByText("No SEPP certificate yet"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Generate" }));
    expect(within(dialog()).getByLabelText(/^FQDN/)).toHaveAttribute(
      "placeholder",
      "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
    );
    await user.click(
      within(dialog()).getByRole("button", { name: "Generate" }),
    );

    expect(
      await screen.findByText("The trust anchor changed"),
    ).toBeInTheDocument();
    expect(sent(`${SEPP_PATH}/generate`, "POST")).toEqual({});
  });

  it("asks for confirmation before regenerating and rejects a foreign FQDN", async () => {
    const user = userEvent.setup();
    renderRoaming("/roaming/sepp");

    await user.click(await screen.findByRole("button", { name: "Regenerate" }));
    expect(
      within(dialog()).getByText(
        /every roaming partner must import the new one/,
      ),
    ).toBeInTheDocument();

    await setText(user, /^FQDN/, "sepp.example.org");
    expect(
      await within(dialog()).findByText(
        "FQDN must be under 5gc.mnc001.mcc001.3gppnetwork.org",
      ),
    ).toBeInTheDocument();

    await setText(user, /^FQDN/, "sepp.5gc.mnc001.mcc001.3gppnetwork.org");
    expect(
      await within(dialog()).findByText(/the well-known sepp\./),
    ).toBeInTheDocument();
  });

  it("imports a certificate and shows the API warnings", async () => {
    const user = userEvent.setup();
    api.put(SEPP_PATH, () => ({
      ...certificate(),
      warnings: ["The certificate is valid for more than 3 years"],
    }));
    renderRoaming("/roaming/sepp");

    await user.click(await screen.findByRole("button", { name: "Import" }));
    await setText(user, /^Certificate Chain/, PEM);
    await setText(
      user,
      /^Private Key/,
      "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
    );
    await setText(user, /^FQDN/, "sepp2.5gc.mnc001.mcc001.3gppnetwork.org");
    const importButton = within(dialog()).getByRole("button", {
      name: "Import",
    });
    await waitFor(() => expect(importButton).toBeEnabled());
    await user.click(importButton);

    expect(
      await screen.findByText("The certificate is valid for more than 3 years"),
    ).toBeInTheDocument();
    expect(sent(SEPP_PATH, "PUT")).toEqual({
      certificateChainPEM: PEM,
      privateKeyPEM:
        "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
      fqdn: "sepp2.5gc.mnc001.mcc001.3gppnetwork.org",
    });
  });

  it("hides import and regenerate from read-only users", async () => {
    renderRoaming("/roaming/sepp", "Read Only");

    await screen.findByText("sha256:ab12cd34");
    expect(
      screen.queryByRole("button", { name: "Regenerate" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Import" }),
    ).not.toBeInTheDocument();
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
