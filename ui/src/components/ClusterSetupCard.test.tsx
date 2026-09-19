// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import ClusterSetupCard from "./ClusterSetupCard";

const api = setupApiServer();

const JOIN = "/api/v1/cluster/join";
const BOOTSTRAP = "/api/v1/cluster/bootstrap";

const TOKEN = "a".repeat(100);

describe("ClusterSetupCard", () => {
  it("founds a cluster from this node", async () => {
    const user = userEvent.setup();
    const onSubmitted = vi.fn();
    api.post(BOOTSTRAP, () => ({ state: "joining" }));

    renderWithProviders(
      <ClusterSetupCard state="waiting" onSubmitted={onSubmitted} />,
    );

    await user.click(screen.getByRole("button", { name: /Create a cluster/ }));

    expect(api.lastRequest(BOOTSTRAP)).toBeDefined();
    expect(onSubmitted).toHaveBeenCalled();
  });

  it("warns that joining replaces everything on the node", async () => {
    const user = userEvent.setup();

    renderWithProviders(
      <ClusterSetupCard state="waiting" onSubmitted={vi.fn()} />,
    );

    await user.click(
      screen.getByRole("button", { name: /Join an existing cluster/ }),
    );

    expect(screen.getByText(/replaced by the cluster/i)).toBeInTheDocument();
  });

  it("sends the token and seed address, and blocks Join until both are set", async () => {
    const user = userEvent.setup();
    api.post(JOIN, () => ({ state: "joining" }));

    renderWithProviders(
      <ClusterSetupCard state="waiting" onSubmitted={vi.fn()} />,
    );

    await user.click(
      screen.getByRole("button", { name: /Join an existing cluster/ }),
    );

    const join = screen.getByRole("button", { name: /^Join$/ });
    expect(join).toBeDisabled();

    await user.type(screen.getByLabelText(/Join token/), TOKEN);
    await user.type(screen.getByLabelText(/Cluster address/), "10.0.0.1:7000");

    expect(join).toBeEnabled();
    await user.click(join);

    expect(api.lastRequest(JOIN)?.body).toEqual({
      token: TOKEN,
      seedAddresses: ["10.0.0.1:7000"],
    });
  });

  it("shows why the last attempt failed", () => {
    renderWithProviders(
      <ClusterSetupCard
        state="waiting"
        failure="peer 10.0.0.1:7000 rejected the join"
        onSubmitted={vi.fn()}
      />,
    );

    expect(screen.getByText(/rejected the join/)).toBeInTheDocument();
  });

  it("opens straight into the join form when that is the only option", () => {
    renderWithProviders(
      <ClusterSetupCard state="waiting" joinOnly onSubmitted={vi.fn()} />,
    );

    expect(screen.getByLabelText(/Join token/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Create a cluster/ }),
    ).not.toBeInTheDocument();
  });
});
