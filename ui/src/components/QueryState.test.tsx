// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseQueryResult } from "@tanstack/react-query";
import QueryState from "./QueryState";

const query = <T,>(over: Partial<UseQueryResult<T>>): UseQueryResult<T> =>
  ({
    data: undefined,
    error: null,
    isLoading: false,
    isLoadingError: false,
    isRefetchError: false,
    isFetching: false,
    fetchStatus: "idle",
    dataUpdatedAt: 0,
    refetch: vi.fn(),
    ...over,
  }) as UseQueryResult<T>;

const renderState = (q: UseQueryResult<string[]>, extra = {}) =>
  render(
    <QueryState
      query={q}
      resource="subscribers"
      isEmpty={(d) => d.length === 0}
      empty={<div>No subscribers yet</div>}
      {...extra}
    >
      {(data) => <div>rows: {data.length}</div>}
    </QueryState>,
  );

describe("QueryState branch order", () => {
  it("reports a paused query as offline before anything else", () => {
    renderState(query<string[]>({ fetchStatus: "paused", isLoading: true }));

    expect(screen.getByRole("alert")).toHaveTextContent(/offline/i);
  });

  it("shows the error, not the empty state, when the first fetch fails", () => {
    renderState(
      query<string[]>({
        isLoadingError: true,
        error: new Error("boom"),
        fetchStatus: "idle",
      }),
    );

    expect(screen.getByText("Failed to load subscribers")).toBeInTheDocument();
    expect(screen.queryByText("No subscribers yet")).not.toBeInTheDocument();
  });

  it("offers a retry that calls refetch", async () => {
    const refetch = vi.fn().mockResolvedValue(undefined);
    renderState(
      query<string[]>({
        isLoadingError: true,
        error: new Error("boom"),
        refetch,
      }),
    );

    await userEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(refetch).toHaveBeenCalledOnce();
  });

  it("treats a disabled query with no data as loading, not empty", () => {
    renderState(query<string[]>({ isLoading: false, data: undefined }));

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    expect(screen.queryByText("No subscribers yet")).not.toBeInTheDocument();
  });

  it("shows no-results rather than the empty state when a filter is active", () => {
    renderState(query<string[]>({ data: [] }), {
      filtered: true,
      noResults: <div>No matches</div>,
    });

    expect(screen.getByText("No matches")).toBeInTheDocument();
    expect(screen.queryByText("No subscribers yet")).not.toBeInTheDocument();
  });

  it("keeps the last good data on screen when a background refetch fails", () => {
    renderState(query<string[]>({ data: ["a", "b"], isRefetchError: true }));

    expect(screen.getByText(/rows: 2/)).toBeInTheDocument();
  });
});
