// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Outlet, Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import { stubDiameterStatus } from "@/test/diameter";
import { PRODUCT } from "@/utils/product";
import RoamingTab from "./RoamingTab";

const api = setupApiServer();

beforeEach(() => {
  stubDiameterStatus(api);
});

const PARTNERS_PATH = "/api/v1/integrations/roaming/partners";
const DIAMETER_PEERS_PATH = "/api/v1/integrations/roaming/diameter-peers";
const SEPP_PATH = "/api/v1/integrations/roaming/sepp";
const seppCertPath = (seppID: number | string) =>
  `${SEPP_PATH}/nodes/${seppID}/certificate`;

beforeEach(() => {
  api.get(PARTNERS_PATH, () => ({ items: [] }));
  api.get(DIAMETER_PEERS_PATH, () => ({ items: [] }));
  api.get(SEPP_PATH, () => ({
    listener: { configured: false, listening: false },
    trustAnchors: [],
    certificates: [],
  }));
});

const operator = () => ({
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
  spn: { fullName: "Ella Networks", shortName: "Ella" },
});

const renderTab = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route element={<Outlet context={{ canEdit: role !== "Read Only" }} />}>
        <Route path="/" element={<RoamingTab />} />
      </Route>
    </Routes>,
    { auth: { role } },
  );

const PEM =
  "-----BEGIN CERTIFICATE-----\nMIIBszCCAVmgAwIBAgIUQ==\n-----END CERTIFICATE-----";

const HOME_DOMAIN = "5gc.mnc001.mcc001.3gppnetwork.org";

const hss = {
  id: "0190a000-0000-7000-8000-0000000000d1",
  originHost: "hss.epc.mnc002.mcc208.3gppnetwork.org",
  address: "192.0.2.30",
  port: 3868,
  status: {
    state: "open",
    originHost: "hss.epc.mnc002.mcc208.3gppnetwork.org",
    since: "2026-11-08T12:00:00Z",
  },
};

const dea = {
  id: "0190a000-0000-7000-8000-0000000000d2",
  originHost: "",
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

const rowLabels = async () =>
  (await table("This core"))
    .getAllByRole("row")
    .map((r) => within(r).getAllByRole("cell")[0].textContent);

describe("Integrations roaming tab", () => {
  beforeEach(() => {
    api.get("/api/v1/operator", () => operator());
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

  it("puts the status right before the actions in every table", async () => {
    renderTab();

    const headers = async (name: string, count: number) =>
      (await table(name))
        .getAllByRole("cell")
        .slice(0, count)
        .map((c) => c.textContent);

    expect(await headers("Roaming partners", 6)).toEqual([
      "Name",
      "PLMNs",
      "Profile",
      "Diameter Peer",
      "N32 Status",
      "Actions",
    ]);
    expect(await headers("Diameter peers", 5)).toEqual([
      "Origin-Host",
      "Address",
      "Partners",
      "Status",
      "Actions",
    ]);
  });

  describe("partners", () => {
    it("lists partners with their PLMNs, profile, N32 status and Diameter peer", async () => {
      renderTab();

      const orangeRow = await tableRow("Roaming partners", "Orange");
      expect(orangeRow.getByText("208 / 01, 208 / 02")).toBeInTheDocument();
      expect(orangeRow.getByRole("link", { name: "roamers" })).toHaveAttribute(
        "href",
        "/profiles/roamers",
      );
      expect(orangeRow.getByText("Established")).toBeInTheDocument();

      const sfrRow = await tableRow("Roaming partners", "SFR");
      expect(
        await sfrRow.findByText("hss.epc.mnc002.mcc208.3gppnetwork.org"),
      ).toBeInTheDocument();
      expect(sfrRow.queryByText("Established")).not.toBeInTheDocument();
    });

    it("shows a Diameter peer without an Origin-Host by its address", async () => {
      api.get(PARTNERS_PATH, () => ({
        items: [{ ...sfr, diameterPeerID: dea.id }],
      }));
      renderTab();

      expect(
        (await tableRow("Roaming partners", "SFR")).getByText(
          "[2001:db8::30]:3869",
        ),
      ).toBeInTheDocument();
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
      renderTab();

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
      renderTab();

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
      renderTab();

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
      renderTab();

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
      renderTab();

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

      const create = within(dialog()).getByRole("button", { name: "Add" });
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
      renderTab();

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
        within(dialog()).getByRole("button", { name: "Add" }),
      ).toBeDisabled();
    });

    it("edits the PLMNs and drops 5G when switched off", async () => {
      const user = userEvent.setup();
      api.put(`${PARTNERS_PATH}/:id`, () => undefined);
      renderTab();

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
      renderTab();

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

    it("shows the status of each peer", async () => {
      renderTab();

      expect(
        (await tableRow("Diameter peers", "192.0.2.30:3868")).getByText(
          "Connected",
        ),
      ).toBeInTheDocument();
      expect(
        (await tableRow("Diameter peers", "[2001:db8::30]:3869")).getByText(
          "Connecting",
        ),
      ).toBeInTheDocument();
    });

    it("adds a peer", async () => {
      const user = userEvent.setup();
      api.post(DIAMETER_PEERS_PATH, () => ({ ...hss, id: "new" }));
      renderTab();

      await user.click(
        await screen.findByRole("button", { name: "Add Diameter Peer" }),
      );
      expect(dialog()).toHaveAccessibleName("Add Diameter Peer");
      await setText(user, /^Address/, "198.51.100.7");
      await setText(user, /^Origin-Host/, "dea.example.org");
      const create = within(dialog()).getByRole("button", { name: "Add" });
      await waitFor(() => expect(create).toBeEnabled());
      await user.click(create);

      await waitFor(() =>
        expect(sent(DIAMETER_PEERS_PATH, "POST")).toEqual({
          originHost: "dea.example.org",
          address: "198.51.100.7",
          port: 3868,
        }),
      );
    });

    it("deletes an unused peer", async () => {
      const user = userEvent.setup();
      api.delete(`${DIAMETER_PEERS_PATH}/:id`, () => undefined);
      renderTab();

      await user.click(
        await screen.findByRole("button", {
          name: "Delete Diameter peer [2001:db8::30]:3869",
        }),
      );
      expect(dialog()).toHaveTextContent(
        "Delete the Diameter peer [2001:db8::30]:3869? Its connection is closed.",
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
      renderTab("Read Only");

      const cert = await tableRow("This core", "Certificate");
      expect(
        cert.getByText((text) =>
          text.startsWith(`sepp1.${HOME_DOMAIN} · Generated · Expires `),
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText("Listening")).not.toBeInTheDocument();
      expect(cert.queryByText(/^Expire/)).not.toBeInTheDocument();

      const anchor = await tableRow("This core", "Trust anchor");
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

      expect(await rowLabels()).toEqual([
        "N32 Address",
        "Certificate",
        "Trust anchor",
        "Diameter Realm",
        "Origin-Host",
      ]);
      expect(
        screen.queryByRole("table", { name: "Trust anchors" }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(`sepp2.${HOME_DOMAIN}`),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("sha256:ca11")).not.toBeInTheDocument();
    });

    it("shows the N32 address to give to partners", async () => {
      renderTab("Read Only");

      await screen.findByText("192.0.2.5:443");
      const n32 = await tableRow("This core", "N32 Address");
      expect(
        n32.getByRole("button", { name: "Copy N32 address" }),
      ).toBeInTheDocument();
    });

    it("shows why the N32 listener is down", async () => {
      api.get(SEPP_PATH, () => ({
        ...seppStatus(),
        listener: {
          seppID: 1,
          configured: true,
          address: "192.0.2.5",
          port: 443,
          listening: false,
          error: "address already in use",
        },
      }));
      const user = userEvent.setup();
      renderTab();

      await user.hover(
        (await tableRow("This core", "N32 Address")).getByText("Not listening"),
      );
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "address already in use",
      );
    });

    it("shows the SEPP as disabled when this node has no N32 interface", async () => {
      api.get(SEPP_PATH, () => ({
        ...seppStatus(),
        listener: { configured: false, listening: false },
      }));
      const user = userEvent.setup();
      renderTab();

      const sepp = await tableRow("This core", "N32 Address");
      await user.hover(await sepp.findByText("Not listening"));
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "Set interfaces.n32 in the configuration file.",
      );
      expect(
        sepp.getByRole("link", { name: "Learn how to enable roaming" }),
      ).toHaveAttribute("href", PRODUCT.roamingDocsUrl);
      expect(
        sepp.queryByRole("button", { name: "Copy N32 address" }),
      ).not.toBeInTheDocument();
      expect(await rowLabels()).toEqual([
        "N32 Address",
        "Diameter Realm",
        "Origin-Host",
      ]);
      expect(
        (await tableRow("This core", "Diameter Realm")).getByText(
          "epc.mnc001.mcc001.3gppnetwork.org",
        ),
      ).toBeInTheDocument();
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
      renderTab();

      expect(
        (await tableRow("This core", "Certificate")).getByText(
          "Expires in 10 days",
        ),
      ).toBeInTheDocument();
    });

    it("warns about an expired certificate", async () => {
      api.get(SEPP_PATH, () => seppStatus(days(-2)));
      renderTab();

      expect(
        (await tableRow("This core", "Certificate")).getByText("Expired"),
      ).toBeInTheDocument();
    });

    it("flags this node's SEPP FQDN outside the operator PLMN", async () => {
      api.get(SEPP_PATH, () =>
        seppStatus(days(1000), "sepp1.5gc.mnc001.mcc208.3gppnetwork.org"),
      );
      renderTab();

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
      renderTab();

      await tableRow("This core", "Certificate");
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
      renderTab("Read Only");

      const button = (await tableRow("This core", "Trust anchor")).getByRole(
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
      renderTab();

      await user.click(
        (await tableRow("This core", "Certificate")).getByRole("button", {
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
      renderTab();

      await user.click(
        (await tableRow("This core", "Certificate")).getByRole("button", {
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
      renderTab();

      await user.click(
        (await tableRow("This core", "Certificate")).getByRole("button", {
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
      renderTab();

      await user.click(
        (await tableRow("This core", "Trust anchor")).getByRole("button", {
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
      renderTab();

      for (const [rowName, name] of [
        ["Certificate", "Regenerate certificate"],
        ["Trust anchor", "Regenerate trust anchor"],
      ]) {
        await user.click(
          (await tableRow("This core", rowName)).getByRole("button", { name }),
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
    renderTab("Read Only");

    await tableRow("Roaming partners", "Orange");
    await tableRow("This core", "Certificate");
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
