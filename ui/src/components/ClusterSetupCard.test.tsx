// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
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
    api.get(JOIN, () => ({ state: "joined" }));

    renderWithProviders(
      <ClusterSetupCard state="waiting" onSubmitted={onSubmitted} />,
    );

    await user.click(screen.getByRole("button", { name: /Create a cluster/ }));

    expect(api.lastRequest(BOOTSTRAP)).toBeDefined();
    await waitFor(() => expect(onSubmitted).toHaveBeenCalled());
  });

  it("sends the token and seed address, and blocks Join until both are set", async () => {
    const user = userEvent.setup();
    api.post(JOIN, () => ({ state: "joining" }));
    api.get(JOIN, () => ({ state: "joined" }));

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

    const posted = api
      .requests(JOIN)
      .filter((r) => r.method === "POST")
      .at(-1);
    expect(posted?.body).toEqual({
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

  it("stays on the join screen and reports progress while the join runs", async () => {
    const user = userEvent.setup();
    const onSubmitted = vi.fn();
    api.post(JOIN, () => ({ state: "joining" }));
    api.get(JOIN, () => ({ state: "joining" }));

    renderWithProviders(
      <ClusterSetupCard state="waiting" joinOnly onSubmitted={onSubmitted} />,
    );

    await user.type(screen.getByLabelText(/Join token/), TOKEN);
    await user.type(screen.getByLabelText(/Cluster address/), "10.0.0.1:7000");
    await user.click(screen.getByRole("button", { name: /^Join$/ }));

    await waitFor(() =>
      expect(screen.getByRole("status")).toHaveTextContent(
        /Joining the cluster/,
      ),
    );
    expect(onSubmitted).not.toHaveBeenCalled();
  });

  it("surfaces the reason a join failed and returns to the form", async () => {
    const user = userEvent.setup();
    api.post(JOIN, () => ({ state: "joining" }));
    api.get(JOIN, () => ({
      state: "waiting",
      error: "peer 10.0.0.1:7000 rejected the join",
    }));

    renderWithProviders(
      <ClusterSetupCard state="waiting" joinOnly onSubmitted={vi.fn()} />,
    );

    await user.type(screen.getByLabelText(/Join token/), TOKEN);
    await user.type(screen.getByLabelText(/Cluster address/), "10.0.0.1:7000");
    await user.click(screen.getByRole("button", { name: /^Join$/ }));

    await waitFor(() =>
      expect(screen.getByText(/rejected the join/)).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: /^Join$/ })).toBeEnabled();
  });
});
