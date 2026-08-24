// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { useEffect, useState } from "react";
import type { GridPaginationModel } from "@mui/x-data-grid";
import { useFilteredPagination } from "./useFilteredPagination";

let changeUser: (v: string) => void;
let paginate: React.Dispatch<React.SetStateAction<GridPaginationModel>>;
const requested: number[] = [];

const goToPage = (p: number) => paginate((prev) => ({ ...prev, page: p }));

const Grid = () => {
  const [user, setUserState] = useState("");
  const [pagination, setPagination] = useFilteredPagination({ user });

  useEffect(() => {
    changeUser = setUserState;
    paginate = setPagination;
  }, [setPagination]);

  useEffect(() => {
    requested.push(pagination.page);
    // oxlint-disable-next-line react/exhaustive-effect-dependencies
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

    act(() => goToPage(5));
    expect(screen.getByTestId("page")).toHaveTextContent("5");

    act(() => changeUser("admin@ellanetworks.com"));
    expect(screen.getByTestId("page")).toHaveTextContent("0");
  });

  it("never renders the stale page against the new filter", () => {
    render(<Grid />);
    act(() => goToPage(5));

    requested.length = 0;
    act(() => changeUser("admin@ellanetworks.com"));
    expect(requested).not.toContain(5);
    expect(requested).toContain(0);
  });

  it("keeps the page when an unrelated re-render produces an equal filter", () => {
    render(<Grid />);
    act(() => goToPage(3));

    act(() => changeUser(""));

    expect(screen.getByTestId("page")).toHaveTextContent("3");
  });

  it("preserves the chosen page size across a reset", () => {
    render(<Grid />);
    act(() => goToPage(4));
    act(() => changeUser("someone@ellanetworks.com"));

    expect(screen.getByTestId("page")).toHaveTextContent("0");
  });
});
