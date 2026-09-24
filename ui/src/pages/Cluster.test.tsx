// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import ClusterPage from "./Cluster";
import { PRODUCT } from "@/utils/product";

const api = setupApiServer();

const STATUS = "/api/v1/status";
const JOIN = "/api/v1/cluster/join";
const MEMBERS = "/api/v1/cluster/members";
const AUTOPILOT = "/api/v1/cluster/autopilot";

type MemberOverrides = {
  nodeId: number | string;
  displayName?: string;
  drainState?: "active" | "draining" | "drained";
  isLeader?: boolean;
  binaryVersion?: string;
};

const member = (o: MemberOverrides) => ({
  nodeId: o.nodeId,
  displayName: o.displayName ?? "",
  raftAddress: `10.0.0.${o.nodeId}:7000`,
  apiAddress: `10.0.0.${o.nodeId}:5002`,
  binaryVersion: o.binaryVersion ?? "v0.11.8",
  suffrage: "voter",
  isLeader: o.isLeader ?? false,
  drainState: o.drainState ?? "active",
});

const seedJoin = (state = "unavailable", error?: string) =>
  api.get(JOIN, () => ({ state, ...(error ? { error } : {}) }));

const seedStatus = (cluster: Record<string, unknown> = {}) => {
  seedJoin();

  return api.get(STATUS, () => ({
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
};

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

const row = (nodeId: number | string) =>
  screen.getByRole("row", { name: new RegExp(`^${nodeId}\\b`) });

const openActions = async (rowEl: HTMLElement) => {
  await userEvent.click(within(rowEl).getByRole("button", { name: /more/i }));
  return screen.findByRole("menu");
};

const actionItem = async (rowEl: HTMLElement, name: RegExp) =>
  within(await openActions(rowEl)).getByRole("menuitem", { name });

const removeItem = async (nodeId: number | string) =>
  actionItem(await waitFor(() => row(nodeId)), /Remove from Cluster/);

const closeMenu = () => userEvent.keyboard("{Escape}");

const dialog = () => screen.getByRole("dialog");

const stateCard = () =>
  screen
    .getByRole("heading", { name: "State" })
    .closest(".MuiCard-root") as HTMLElement;

describe("Cluster page standalone state", () => {
  const seedStandalone = () => {
    api.get(STATUS, () => ({
      initialized: true,
      ready: true,
      schemaVersion: 7,
      cluster: { enabled: false },
    }));
    seedJoin();
  };

  it("explains standalone mode and links out to the HA documentation", async () => {
    seedStandalone();

    await renderCluster();

    expect(screen.getByText("High availability is not enabled")).toBeTruthy();
    expect(
      screen.getByText(/cluster bind address in the config file/),
    ).toBeTruthy();

    const learnMore = screen.getByRole("link", { name: /Learn more/ });
    expect(learnMore).toHaveAttribute("href", PRODUCT.haDocsUrl);
    expect(learnMore).toHaveAttribute("target", "_blank");
  });
});

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

  it("reports a leaderless cluster whose node identities are UUIDs", async () => {
    seedStatus({
      nodeId: "0199c0de-0000-7000-8000-00000000beef",
      leaderNodeId: "",
      isLeader: false,
      role: "Follower",
    });
    api.get(AUTOPILOT, () => httpError(503, "no leader"));
    api.get(MEMBERS, () => [
      member({ nodeId: "0199c0de-0000-7000-8000-00000000beef" }),
    ]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("No leader"),
    ).toBeInTheDocument();
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

  it("names the blocking node by its display name when it has one", async () => {
    seedStatus({
      appliedSchemaVersion: 6,
      pendingMigration: {
        currentSchema: 6,
        targetSchema: 6,
        laggardNodeId: 3,
      },
    });
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 3, displayName: "edge-rack-3" }),
    ]);

    await renderCluster();

    expect(
      await within(stateCard()).findByText("v6 · blocked by node edge-rack-3"),
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

describe("Cluster page load failures", () => {
  it("reports a failed member list instead of rendering an empty table", async () => {
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => httpError(500, "Failed to list cluster members"));

    await renderCluster();

    expect(
      await screen.findByText("Failed to load cluster members"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Failed to list cluster members"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("grid")).not.toBeInTheDocument();
  });

  it("explains why health is unknown when autopilot cannot be read", async () => {
    const user = userEvent.setup();
    seedStatus();
    api.get(AUTOPILOT, () => httpError(500, "forward to leader failed"));
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    const unknown = await within(stateCard()).findByText("Unknown");
    await user.hover(unknown);

    expect(
      await screen.findByRole("tooltip", {
        name: /Health could not be read: forward to leader failed/,
      }),
    ).toBeInTheDocument();
  });
});

describe("Cluster page members table", () => {
  it("shows each node's cluster address", async () => {
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2 }),
    ]);

    await renderCluster();

    await waitFor(() =>
      expect(within(row(2)).getByText("10.0.0.2:7000")).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("columnheader", { name: "Cluster Address" }),
    ).toBeInTheDocument();
  });
});

describe("Cluster page row actions", () => {
  it("offers only the actions that apply to each node", async () => {
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2, drainState: "drained" }),
      { ...member({ nodeId: 3 }), suffrage: "nonvoter" },
    ]);

    await renderCluster();

    const active = await openActions(await waitFor(() => row(1)));
    expect(
      within(active).getByRole("menuitem", { name: /Drain this node/ }),
    ).toBeInTheDocument();
    expect(
      within(active).queryByRole("menuitem", { name: /Resume/ }),
    ).not.toBeInTheDocument();
    expect(
      within(active).queryByRole("menuitem", { name: /Promote/ }),
    ).not.toBeInTheDocument();
    await closeMenu();

    const drained = await openActions(row(2));
    expect(
      within(drained).getByRole("menuitem", { name: /Resume this node/ }),
    ).toBeInTheDocument();
    expect(
      within(drained).queryByRole("menuitem", { name: /Drain/ }),
    ).not.toBeInTheDocument();
    await closeMenu();

    const nonvoter = await openActions(row(3));
    expect(
      within(nonvoter).getByRole("menuitem", { name: /Promote to Voter/ }),
    ).toBeInTheDocument();
  });
});

describe("Cluster page drain", () => {
  it("warns when draining the only active node and reports the result by name", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot();
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true, drainState: "drained" }),
      member({ nodeId: 2, displayName: "edge-rack-2" }),
    ]);
    api.post(`${MEMBERS}/:id/drain`, () => ({ drainState: "draining" }));

    await renderCluster();

    const edgeRow = (await screen.findByText("edge-rack-2")).closest(
      "[role='row']",
    ) as HTMLElement;
    await user.click(await actionItem(edgeRow, /Drain this node/));

    expect(
      within(dialog()).getByText(/subscribers have nowhere to move/),
    ).toBeInTheDocument();
    expect(
      within(dialog()).queryByText(/Leadership will move/),
    ).not.toBeInTheDocument();

    await user.click(within(dialog()).getByRole("button", { name: /^Drain$/ }));

    expect(
      await screen.findByText(
        "Node edge-rack-2 is draining. Subscribers are being moved.",
      ),
    ).toBeInTheDocument();
  });
});

describe("Cluster page node health", () => {
  it("explains a missing health value when autopilot cannot be read", async () => {
    const user = userEvent.setup();
    seedStatus();
    api.get(AUTOPILOT, () => httpError(500, "forward to leader failed"));
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    const placeholder = await waitFor(() => within(row(1)).getByText("—"));
    await user.hover(placeholder);

    expect(
      await screen.findByRole("tooltip", {
        name: /Health could not be read: forward to leader failed/,
      }),
    ).toBeInTheDocument();
  });

  it("says the leader has not reported when autopilot omits the node", async () => {
    const user = userEvent.setup();
    seedStatus();
    seedAutopilot({ servers: [] });
    api.get(MEMBERS, () => [member({ nodeId: 1, isLeader: true })]);

    await renderCluster();

    const placeholder = await waitFor(() => within(row(1)).getByText("—"));
    await user.hover(placeholder);

    expect(
      await screen.findByRole("tooltip", {
        name: /The leader has not reported on this node yet/,
      }),
    ).toBeInTheDocument();
  });
});

describe("Cluster page leader", () => {
  it("marks the leader from cluster status even when the member list lags", async () => {
    seedStatus({ leaderNodeId: 2, isLeader: false, role: "Follower" });
    seedAutopilot({ leaderNodeId: 2 });
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: 2 }),
    ]);

    await renderCluster();

    await waitFor(() =>
      expect(within(row(2)).getByText("Leader")).toBeInTheDocument(),
    );
    expect(within(row(1)).queryByText("Leader")).not.toBeInTheDocument();
    expect(await removeItem(2)).toHaveAttribute("aria-disabled", "true");
    await closeMenu();
    expect(await removeItem(1)).not.toHaveAttribute("aria-disabled");
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

    await user.click(await removeItem(2));

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

    await user.click(await removeItem(2));

    expect(
      within(dialog()).getByRole("checkbox", { name: /Force remove/ }),
    ).not.toBeChecked();
    expect(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    ).toBeDisabled();
  });

  it("requires an explicit opt-in before removing a healthy node", async () => {
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

    await user.click(await removeItem(2));

    const checkbox = within(dialog()).getByRole("checkbox", {
      name: /Force remove/,
    });
    expect(checkbox).not.toBeChecked();
    expect(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    ).toBeDisabled();

    await user.click(checkbox);

    expect(
      within(dialog()).getByRole("button", { name: /^Remove$/ }),
    ).toBeEnabled();
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

    await user.click(await removeItem(2));

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

    await user.click(await removeItem(2));

    expect(within(dialog()).getByText(/ends your session/)).toBeInTheDocument();
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

    await user.click(await removeItem(2));
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

    const remove = await removeItem(1);
    expect(remove).toHaveAttribute("aria-disabled", "true");
    expect(remove).toHaveTextContent(/Drain the leader first/);
  });
});

describe("Cluster page node identity", () => {
  const UUID = "0199c0de-0000-7000-8000-00000000beef";

  it("shows a UUID identity shortened, and the display name where one is set", async () => {
    api.get(MEMBERS, () => [
      member({ nodeId: 1, isLeader: true }),
      member({ nodeId: UUID, displayName: "edge-rack-4" }),
    ]);
    seedStatus();
    seedAutopilot();

    await renderCluster();

    await screen.findByText("edge-rack-4");
    expect(screen.getByText("00000000beef")).toBeInTheDocument();
    expect(screen.queryByText(UUID)).not.toBeInTheDocument();
  });

  it("falls back to the shortened identity when no display name is set", async () => {
    api.get(MEMBERS, () => [member({ nodeId: UUID })]);
    seedStatus();
    seedAutopilot();

    await renderCluster();

    await screen.findByText("00000000beef");
  });

  it("sends a rename to the display-name endpoint", async () => {
    const user = userEvent.setup();
    const RENAME = "/api/v1/cluster/members/:id/display-name";

    api.get(MEMBERS, () => [member({ nodeId: UUID })]);
    seedStatus();
    seedAutopilot();
    api.put(RENAME, () => ({ message: "Display name updated" }));

    await renderCluster();

    const uuidRow = (await screen.findByText("00000000beef")).closest(
      "[role='row']",
    ) as HTMLElement;

    await user.click(await actionItem(uuidRow, /Rename this node/));

    const input = await screen.findByLabelText(/Display name/);
    await user.clear(input);
    await user.type(input, "edge-rack-4");
    await user.click(within(dialog()).getByRole("button", { name: /Save/ }));

    await waitFor(() =>
      expect(
        api.lastRequest(`/api/v1/cluster/members/${UUID}/display-name`)?.body,
      ).toMatchObject({ displayName: "edge-rack-4" }),
    );
  });
});
