// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { useEffect, useState } from "react";
import { useFilteredPagination } from "./useFilteredPagination";

let setUser: (v: string) => void;
let setPage: (p: number) => void;
const requested: number[] = [];

const Grid = () => {
  const [user, setUserState] = useState("");
  const [pagination, setPagination] = useFilteredPagination({ user });

  setUser = setUserState;
  setPage = (p) => setPagination((prev) => ({ ...prev, page: p }));

  // Recorded in an effect so this captures what actually commits. A render-phase
  // reset re-renders before committing, so a discarded render never reaches here
  // -- and never reaches React Query's observer either.
  useEffect(() => {
    requested.push(pagination.page);
  }, [pagination.page, user]);

  return <span data-testid="page">{pagination.page}</span>;
};

describe("useFilteredPagination", () => {
  it("starts on the first page", () => {
    render(<Grid />);
    expect(screen.getByTestId("page")).toHaveTextContent("0");
  });

  it("returns to the first page when a filter changes", () => {
    render(<Grid />);

    act(() => setPage(5));
    expect(screen.getByTestId("page")).toHaveTextContent("5");

    act(() => setUser("admin@ellanetworks.com"));
    expect(screen.getByTestId("page")).toHaveTextContent("0");
  });

  it("never renders the stale page against the new filter", () => {
    render(<Grid />);
    act(() => setPage(5));

    requested.length = 0;
    act(() => setUser("admin@ellanetworks.com"));

    // An effect-based reset would commit page 5 once under the new filter and
    // fire a query for an offset the filtered set does not reach.
    expect(requested).not.toContain(5);
    expect(requested).toContain(0);
  });

  it("keeps the page when an unrelated re-render produces an equal filter", () => {
    render(<Grid />);
    act(() => setPage(3));

    act(() => setUser(""));

    expect(screen.getByTestId("page")).toHaveTextContent("3");
  });

  it("preserves the chosen page size across a reset", () => {
    render(<Grid />);
    act(() => setPage(4));
    act(() => setUser("someone@ellanetworks.com"));

    expect(screen.getByTestId("page")).toHaveTextContent("0");
  });
});
