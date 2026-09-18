// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import ClusterPage from "./Cluster";

const api = setupApiServer();

const STATUS = "/api/v1/status";
const MEMBERS = "/api/v1/cluster/members";
const AUTOPILOT = "/api/v1/cluster/autopilot";

type MemberOverrides = {
  nodeId: number;
  drainState?: "active" | "draining" | "drained";
  isLeader?: boolean;
  binaryVersion?: string;
};

const member = (o: MemberOverrides) => ({
  nodeId: o.nodeId,
  raftAddress: `10.0.0.${o.nodeId}:7000`,
  apiAddress: `10.0.0.${o.nodeId}:5002`,
  binaryVersion: o.binaryVersion ?? "v0.11.8",
  suffrage: "voter",
  isLeader: o.isLeader ?? false,
  drainState: o.drainState ?? "active",
});

const seedStatus = (cluster: Record<string, unknown> = {}) =>
  api.get(STATUS, () => ({
    initialized: true,
    ready: true,
    schemaVersion: 7,
    cluster: {
      enabled: true,
      role: "Leader",
      nodeId: 1,
      isLeader: true,
      leaderNodeId: 1,
      appliedIndex: 42,
      clusterId: "cluster-abc-123",
      appliedSchemaVersion: 7,
      ...cluster,
    },
  }));

const seedAutopilot = (state: Record<string, unknown> = {}) =>
  api.get(AUTOPILOT, () => ({
    healthy: true,
    failureTolerance: 1,
    leaderNodeId: 1,
    voters: [1, 2, 3],
    servers: [],
    ...state,
  }));

const renderCluster = async () => {
  const result = renderWithProviders(<ClusterPage />, { auth: {} });
  await screen.findByRole("heading", { name: /^Cluster$/ });
  return result;
};

const row = (nodeId: number) =>
  screen.getByRole("row", { name: new RegExp(`^${nodeId}\\b`) });

const removeButton = (nodeId: number) =>
  within(row(nodeId)).getByRole("menuitem", { name: /Remove from Cluster/ });

const dialog = () => screen.getByRole("dialog");

const stateCard = () =>
  screen
    .getByRole("heading", { name: "State" })
    .closest(".MuiCard-root") as HTMLElement;

describe("Cluster page State section", () => {
  it("shows cluster id, health with failure tolerance, and schema", async () => {
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("cluster-abc-123"),
    ).toBeInTheDocument();
    expect(await within(stateCard()).findByText("Healthy")).toBeInTheDocument();
    expect(
      within(stateCard()).getByText("Tolerates 1 voter failure"),
    ).toBeInTheDocument();
    expect(within(stateCard()).getByText("v7")).toBeInTheDocument();
  });

  it("warns when the cluster has no failure tolerance left", async () => {
    seedStatus();
    seedAutopilot({ failureTolerance: 0, voters: [1, 2] });
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("No failure tolerance"),
    ).toBeInTheDocument();
  });

  it("reports a leaderless cluster instead of waiting on autopilot", async () => {
    seedStatus({ leaderNodeId: 0, isLeader: false, role: "Follower" });
    api.get(AUTOPILOT, () => httpError(503, "no leader"));
    api.get(MEMBERS, () => [member({ nodeId: 1 })]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("No leader"),
    ).toBeInTheDocument();
    expect(within(stateCard()).queryByText("Unknown")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders an unhealthy cluster as a badge rather than an alert", async () => {
    seedStatus();
    seedAutopilot({ healthy: false });
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("Unhealthy"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("names the node blocking a migration that cannot advance", async () => {
    seedStatus({
      appliedSchemaVersion: 6,
      pendingMigration: {
        currentSchema: 6,
        targetSchema: 6,
        laggardNodeId: 3,
      },
    });
    seedAutopilot();
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("v6 · blocked by node 3"),
    ).toBeInTheDocument();
  });

  it("shows a migration that is still advancing without naming a laggard", async () => {
    seedStatus({
      appliedSchemaVersion: 6,
      pendingMigration: { currentSchema: 6, targetSchema: 7 },
    });
    seedAutopilot();
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    expect(await within(stateCard()).findByText("v6 → v7")).toBeInTheDocument();
    expect(
      within(stateCard()).queryByText(/blocked by/),
    ).not.toBeInTheDocument();
  });

  it("renders health as unknown while autopilot converges, keeping status facts", async () => {
    seedStatus();
    api.get(AUTOPILOT, () => httpError(503, "autopilot has not converged"));
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("cluster-abc-123"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(within(stateCard()).getAllByText("Unknown")).toHaveLength(1),
    );
    expect(within(stateCard()).getByText("v7")).toBeInTheDocument();
  });
});

describe("Cluster page force remove", () => {
  it("enables Remove for an undrained node and sends force=true", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot({
      servers: [
        {
          nodeId: 2,
          raftAddress: "10.0.0.2:7000",
          nodeStatus: "failed",
          healthy: false,
          isLeader: false,
          hasVotingRights: true,
        },
      ],
    });
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "active" }),
    ]);
    api.delete(`${MEMBERS}/:id`, () => undefined);

    await renderCluster();

    await waitFor(() => expect(removeButton(2)).toBeEnabled());
    await user.click(removeButton(2));

    const checkbox = within(dialog()).getByRole("checkbox", {
      name: /Force remove/,
    });
    expect(checkbox).toBeChecked();

    await user.click(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    );

    await waitFor(() =>
      expect(api.lastRequest(`${MEMBERS}/2`)?.params.get("force")).toBe("true"),
    );
  });

  it("leaves force unchecked and blocks confirm when autopilot has no entry for the node", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot({ servers: [] });
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "active" }),
    ]);

    await renderCluster();

    await waitFor(() => expect(removeButton(2)).toBeEnabled());
    await user.click(removeButton(2));

    expect(
      within(dialog()).getByRole("checkbox", { name: /Force remove/ }),
    ).not.toBeChecked();
    expect(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    ).toBeDisabled();
  });

  it("warns what force drops on a healthy node before allowing confirm", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot({
      servers: [
        {
          nodeId: 2,
          raftAddress: "10.0.0.2:7000",
          nodeStatus: "alive",
          healthy: true,
          isLeader: false,
          hasVotingRights: true,
        },
      ],
    });
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "active" }),
    ]);

    await renderCluster();

    await waitFor(() => expect(removeButton(2)).toBeEnabled());
    await user.click(removeButton(2));

    const checkbox = within(dialog()).getByRole("checkbox", {
      name: /Force remove/,
    });
    expect(checkbox).not.toBeChecked();

    await user.click(checkbox);

    expect(
      within(dialog()).getByText(/lose their sessions and must re-attach/),
    ).toBeInTheDocument();
  });

  it("omits the force option for an already drained node", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "drained" }),
    ]);
    api.delete(`${MEMBERS}/:id`, () => undefined);

    await renderCluster();

    await waitFor(() => expect(removeButton(2)).toBeEnabled());
    await user.click(removeButton(2));

    expect(
      within(dialog()).queryByRole("checkbox", { name: /Force remove/ }),
    ).not.toBeInTheDocument();

    await user.click(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    );

    await waitFor(() =>
      expect(api.lastRequest(`${MEMBERS}/2`)?.params.get("force")).toBeNull(),
    );
  });

  it("warns that removing the node serving the page ends the session", async () => {
    const user = userEvent.setup();
    seedStatus({ nodeId: 2, isLeader: false, leaderNodeId: 1 });
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "drained" }),
    ]);

    await renderCluster();

    await waitFor(() => expect(removeButton(2)).toBeEnabled());
    await user.click(removeButton(2));

    expect(
      within(dialog()).getByText(/ends your session here/),
    ).toBeInTheDocument();
  });

  it("keeps the dialog open and shows why when the cluster refuses the removal", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot({ servers: [] });
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "active" }),
    ]);
    api.delete(`${MEMBERS}/:id`, () =>
      httpError(409, "Node is not drained (state=active); drain it first"),
    );

    await renderCluster();

    await waitFor(() => expect(removeButton(2)).toBeEnabled());
    await user.click(removeButton(2));
    await user.click(
      within(dialog()).getByRole("checkbox", { name: /Force remove/ }),
    );
    await user.click(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    );

    expect(
      await within(dialog()).findByText(/Node is not drained/),
    ).toBeInTheDocument();
    expect(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    ).toBeEnabled();
  });

  it("keeps Remove disabled for the current leader", async () => {
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2 }),
    ]);

    await renderCluster();

    await waitFor(() => expect(removeButton(1)).toBeDisabled());
  });
});
