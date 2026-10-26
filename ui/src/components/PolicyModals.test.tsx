// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreatePolicyModal from "./CreatePolicyModal";
import EditPolicyModal from "./EditPolicyModal";
import type { APIPolicy } from "@/queries/policies";

const api = setupApiServer();

const POLICIES = "/api/v1/policies";
const DATA_NETWORKS = "/api/v1/networking/data-networks";
const SLICES = "/api/v1/slices";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);

const page = <T,>(items: T[]) => ({
  items,
  page: 1,
  per_page: 12,
  total_count: items.length,
});

const seed = () => {
  api.get(DATA_NETWORKS, () =>
    page([
      { name: "internet", ims: false },
      { name: "ims", ims: true },
    ]),
  );
  api.get(SLICES, () => page([{ name: "slice-a" }, { name: "slice-b" }]));
};

describe("CreatePolicyModal", () => {
  const render = (policyCount = 1) => {
    const onClose = vi.fn();
    renderWithProviders(
      <CreatePolicyModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        profileName="gold"
        policyCount={policyCount}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("preselects the first slice and data network once loaded", async () => {
    seed();
    render();

    await waitFor(() =>
      expect(screen.getByText("slice-a")).toBeInTheDocument(),
    );
    expect(screen.getByText("internet")).toBeInTheDocument();
    expect(field(/5QI/)).toBeDefined();
  });

  it("reports a dropdown load failure", async () => {
    api.get(DATA_NETWORKS, () => httpError(500, "nope"));
    api.get(SLICES, () => httpError(500, "nope"));
    render();

    await screen.findByText(
      "Failed to load dropdown data. Please close and try again.",
    );
  });

  it("forces the first policy in a profile to be the default", async () => {
    const user = userEvent.setup();
    seed();
    api.post(POLICIES, () => ({}));
    const { onClose } = render(0);

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).toBeChecked();
    expect(checkbox).toBeDisabled();

    await user.type(field(/Name/), "default-policy");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(POLICIES)?.body).toMatchObject({
      name: "default-policy",
      profile_name: "gold",
      default: true,
    });
  });

  it("leaves the default checkbox editable for later policies", async () => {
    seed();
    render(2);

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toBeEnabled();
  });

  it("rejects an ARP outside 1-15", async () => {
    const user = userEvent.setup();
    seed();
    render();

    await user.type(field(/Name/), "p1");
    fireEvent.change(field(/Allocation and Retention Priority/), {
      target: { value: "20" },
    });
    fireEvent.blur(field(/Allocation and Retention Priority/));

    await waitFor(() => expect(button(/^Create$/)).toBeDisabled());
  });

  it("refuses pre-emption with ARP 15", async () => {
    const user = userEvent.setup();
    seed();
    render();

    await user.type(field(/Name/), "p1");
    fireEvent.change(field(/Allocation and Retention Priority/), {
      target: { value: "15" },
    });
    fireEvent.blur(field(/Allocation and Retention Priority/));

    await user.click(
      screen.getByRole("combobox", { name: /Pre-emption vulnerability/ }),
    );
    await user.click(
      await screen.findByRole("option", { name: "Pre-emptable" }),
    );

    await waitFor(() => expect(button(/^Create$/)).toBeDisabled());
  });

  it("limits 5QI to 5 and 69 on an IMS data network", async () => {
    const user = userEvent.setup();
    seed();
    api.post(POLICIES, () => ({}));
    const { onClose } = render(1);

    await user.type(field(/Name/), "voice");
    await waitFor(() =>
      expect(screen.getByText("internet")).toBeInTheDocument(),
    );

    await user.click(screen.getByRole("combobox", { name: /Data Network/ }));
    await user.click(await screen.findByRole("option", { name: "ims" }));

    const fiveQi = screen.getByRole("combobox", { name: /5QI/ });
    await waitFor(() => expect(fiveQi).toHaveTextContent("5 — IMS signalling"));
    expect(
      screen.getByText(
        "IMS signalling uses 5QI 5, or 69 for mission critical services.",
      ),
    ).toBeInTheDocument();

    await user.click(fiveQi);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["5 — IMS signalling", "69 — Mission critical signalling"]);
    await user.click(
      screen.getByRole("option", { name: /69 — Mission critical/ }),
    );

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(POLICIES)?.body).toMatchObject({
      data_network_name: "ims",
      var5qi: 69,
    });
  });

  it("offers every 5QI when leaving the IMS data network", async () => {
    const user = userEvent.setup();
    seed();
    render(1);

    await waitFor(() =>
      expect(screen.getByText("internet")).toBeInTheDocument(),
    );
    await user.click(screen.getByRole("combobox", { name: /Data Network/ }));
    await user.click(await screen.findByRole("option", { name: "ims" }));
    await user.click(screen.getByRole("combobox", { name: /Data Network/ }));
    await user.click(await screen.findByRole("option", { name: "internet" }));

    await user.click(screen.getByRole("combobox", { name: /5QI/ }));
    expect(
      await screen.findByRole("option", { name: /9 — Best effort/ }),
    ).toBeInTheDocument();
  });

  it("lists every data network", async () => {
    const user = userEvent.setup();
    const names = Array.from({ length: 15 }, (_, i) => ({ name: `dn-${i}` }));
    api.get(DATA_NETWORKS, () => page(names));
    api.get(SLICES, () => page([{ name: "slice-a" }]));
    render(1);

    await waitFor(() => expect(screen.getByText("dn-0")).toBeInTheDocument());
    await user.click(screen.getByRole("combobox", { name: /Data Network/ }));
    expect(
      await screen.findByRole("option", { name: "dn-14" }),
    ).toBeInTheDocument();
  });

  it("submits the assembled policy payload", async () => {
    const user = userEvent.setup();
    seed();
    api.post(POLICIES, () => ({}));
    const { onClose } = render(1);

    await user.type(field(/Name/), "video");
    await waitFor(() =>
      expect(screen.getByText("slice-a")).toBeInTheDocument(),
    );

    await user.click(screen.getByRole("combobox", { name: /5QI/ }));
    await user.click(await screen.findByRole("option", { name: /7 — Voice/ }));

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(POLICIES)?.body).toMatchObject({
      name: "video",
      slice_name: "slice-a",
      data_network_name: "internet",
      session_ambr_uplink: "100 Mbps",
      session_ambr_downlink: "100 Mbps",
      var5qi: 7,
      arp: 1,
      preemption_capability: "not_preempt",
      preemption_vulnerability: "not_preemptable",
      default: false,
    });
  });
});

describe("EditPolicyModal", () => {
  const initialData = {
    name: "video",
    profile_name: "gold",
    slice_name: "slice-a",
    data_network_name: "internet",
    var5qi: 7,
    arp: 3,
    preemption_capability: "may_preempt",
    preemption_vulnerability: "preemptable",
    default: false,
  } as APIPolicy;

  const full = {
    ...initialData,
    session_ambr_uplink: "20 Mbps",
    session_ambr_downlink: "2 Gbps",
    rules: { some: "rules" },
  };

  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditPolicyModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("seeds from the fetched policy and locks the name", async () => {
    seed();
    api.get(`${POLICIES}/:name`, () => full);
    render();

    await waitFor(() =>
      expect(field(/Session Bitrate Uplink/)).toHaveValue(20),
    );
    expect(field(/Session Bitrate Downlink/)).toHaveValue(2);
    expect(field(/Name/)).toHaveValue("video");
    expect(field(/Name/)).toBeDisabled();
    expect(field(/Allocation and Retention Priority/)).toHaveValue(3);
  });

  it("preserves the policy rules it did not edit", async () => {
    const user = userEvent.setup();
    seed();
    api.get(`${POLICIES}/:name`, () => full);
    api.put(`${POLICIES}/:name`, () => ({}));
    const { onClose } = render();

    await waitFor(() =>
      expect(field(/Session Bitrate Uplink/)).toHaveValue(20),
    );
    fireEvent.change(field(/Session Bitrate Uplink/), {
      target: { value: "40" },
    });

    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(`${POLICIES}/video`)?.body).toMatchObject({
      profile_name: "gold",
      session_ambr_uplink: "40 Mbps",
      session_ambr_downlink: "2 Gbps",
      rules: { some: "rules" },
      var5qi: 7,
      preemption_capability: "may_preempt",
      preemption_vulnerability: "preemptable",
    });
  });

  it("moves an IMS policy with another 5QI to 5", async () => {
    seed();
    api.get(`${POLICIES}/:name`, () => ({
      ...full,
      data_network_name: "ims",
      var5qi: 9,
    }));
    render();

    const fiveQi = await screen.findByRole("combobox", { name: /5QI/ });
    await waitFor(() => expect(fiveQi).toHaveTextContent("5 — IMS signalling"));
  });

  it("keeps 5QI 69 on an IMS policy", async () => {
    seed();
    api.get(`${POLICIES}/:name`, () => ({
      ...full,
      data_network_name: "ims",
      var5qi: 69,
    }));
    render();

    const fiveQi = await screen.findByRole("combobox", { name: /5QI/ });
    await waitFor(() =>
      expect(fiveQi).toHaveTextContent("69 — Mission critical signalling"),
    );
  });

  it("keeps a 5QI value that is not in the standard list selectable", async () => {
    seed();
    api.get(`${POLICIES}/:name`, () => ({ ...full, var5qi: 42 }));
    render();

    await waitFor(() => expect(screen.getByText("42")).toBeInTheDocument());
  });
});
