// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import PolicyRulesModal from "./PolicyRulesModal";
import type { APIPolicy } from "@/queries/policies";

const api = setupApiServer();

const POLICY_PATH = "/api/v1/policies/:name";

const rulesDialog = () =>
  screen.findByRole("dialog", { name: /Edit Uplink Rules/ });
const ruleForm = () => screen.getByRole("dialog", { name: /Rule$/ });

const policy = {
  name: "video",
  profile_name: "gold",
  slice_name: "slice-a",
  data_network_name: "internet",
  session_ambr_uplink: "20 Mbps",
  session_ambr_downlink: "2 Gbps",
  var5qi: 7,
  arp: 3,
  rules: {
    uplink: [
      {
        description: "allow web",
        action: "allow",
        remote_prefix: "10.0.0.0/8",
        protocol: 6,
        port_low: 80,
        port_high: 80,
      },
      {
        description: "deny rest",
        action: "deny",
        protocol: 0,
        port_low: 0,
        port_high: 0,
      },
    ],
    downlink: [
      {
        description: "dl rule",
        action: "allow",
        protocol: 0,
        port_low: 0,
        port_high: 0,
      },
    ],
  },
} as unknown as APIPolicy;

const render = (direction: "uplink" | "downlink" = "uplink") => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <PolicyRulesModal
      open
      onClose={onClose}
      onSuccess={onSuccess}
      policy={policy}
      direction={direction}
    />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

const savedRules = () => {
  const request = api.lastRequest("/api/v1/policies/video");
  if (!request) throw new Error("the policy was never saved");
  return (request.body as Record<string, unknown>).rules as {
    uplink?: unknown[];
    downlink?: unknown[];
  };
};

describe("PolicyRulesModal", () => {
  it("lists the existing rules for the direction, in order", () => {
    render();
    expect(screen.getByText("allow web")).toBeInTheDocument();
    expect(screen.getByText("deny rest")).toBeInTheDocument();
    expect(screen.queryByText("dl rule")).not.toBeInTheDocument();
    expect(screen.getByText("10.0.0.0/8")).toBeInTheDocument();
  });

  it("shows an empty state when the direction has no rules", () => {
    renderWithProviders(
      <PolicyRulesModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        policy={{ ...policy, rules: {} } as APIPolicy}
        direction="uplink"
      />,
      { auth: {} },
    );
    expect(screen.getByText("No uplink rules configured.")).toBeInTheDocument();
  });

  it("requires a description before a rule can be added", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getByRole("button", { name: /Add Rule/ }));
    const form = ruleForm();
    expect(within(form).getByRole("button", { name: /^Add$/ })).toBeDisabled();

    await user.type(within(form).getByLabelText(/Description/), "new rule");
    await waitFor(() =>
      expect(within(form).getByRole("button", { name: /^Add$/ })).toBeEnabled(),
    );
  });

  it("rejects an invalid remote prefix", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getByRole("button", { name: /Add Rule/ }));
    const form = ruleForm();
    await user.type(within(form).getByLabelText(/Description/), "r");
    await user.type(within(form).getByLabelText(/Remote Prefix/), "nope");
    await user.tab();

    await screen.findByText(/Must be valid CIDR format/);
    expect(within(form).getByRole("button", { name: /^Add$/ })).toBeDisabled();
  });

  it("adds a rule to the list without calling the API", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getByRole("button", { name: /Add Rule/ }));
    const form = ruleForm();
    await user.type(within(form).getByLabelText(/Description/), "third rule");
    await waitFor(() =>
      expect(within(form).getByRole("button", { name: /^Add$/ })).toBeEnabled(),
    );
    await user.click(within(form).getByRole("button", { name: /^Add$/ }));

    await waitFor(() =>
      expect(screen.getByText("third rule")).toBeInTheDocument(),
    );
    expect(api.requests("/api/v1/policies/video")).toHaveLength(0);
  });

  it("seeds the edit form from the selected rule", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getAllByTitle("Edit rule")[0]);
    const form = ruleForm();

    expect(within(form).getByLabelText(/Description/)).toHaveValue("allow web");
    expect(within(form).getByLabelText(/Remote Prefix/)).toHaveValue(
      "10.0.0.0/8",
    );
    expect(within(form).getByLabelText(/Port Low/)).toHaveValue(80);
    expect(
      within(form).getByRole("button", { name: /^Update$/ }),
    ).toBeInTheDocument();
  });

  it("deletes a rule from the list", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getAllByTitle("Delete rule")[0]);

    await waitFor(() =>
      expect(screen.queryByText("allow web")).not.toBeInTheDocument(),
    );
    expect(screen.getByText("deny rest")).toBeInTheDocument();
  });

  it("saves the edited direction and preserves the other one", async () => {
    const user = userEvent.setup();
    api.put(POLICY_PATH, () => ({}));
    const { onClose, onSuccess } = render();

    await user.click(screen.getAllByTitle("Delete rule")[1]);
    await user.click(
      within(await rulesDialog()).getByRole("button", { name: /^Save$/ }),
    );

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();

    const rules = savedRules();
    expect(rules.uplink).toHaveLength(1);
    expect(rules.uplink?.[0]).toMatchObject({ description: "allow web" });
    expect(rules.downlink).toHaveLength(1);
  });

  it("converts a protocol name back to its number on save", async () => {
    const user = userEvent.setup();
    api.put(POLICY_PATH, () => ({}));
    render();

    await user.click(screen.getByRole("button", { name: /Add Rule/ }));
    const form = ruleForm();
    await user.type(within(form).getByLabelText(/Description/), "udp rule");
    await user.click(within(form).getByLabelText(/Protocol/));
    await user.click(await screen.findByRole("option", { name: /UDP \(17\)/ }));
    await user.click(within(form).getByRole("button", { name: /^Add$/ }));

    await waitFor(() =>
      expect(screen.getByText("udp rule")).toBeInTheDocument(),
    );
    await user.click(
      within(await rulesDialog()).getByRole("button", { name: /^Save$/ }),
    );

    await waitFor(() => expect(savedRules().uplink).toHaveLength(3));
    expect(savedRules().uplink?.[2]).toMatchObject({
      description: "udp rule",
      protocol: 17,
    });
  });

  it("keeps the dialog open and reports a failed save", async () => {
    const user = userEvent.setup();
    api.put(POLICY_PATH, () => httpError(500, "rules rejected"));
    const { onClose } = render();

    await user.click(
      within(await rulesDialog()).getByRole("button", { name: /^Save$/ }),
    );

    await screen.findByText(/Failed to save rules: .*rules rejected/);
    expect(onClose).not.toHaveBeenCalled();
  });
});
