// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import type { GridPaginationModel } from "@mui/x-data-grid";

/**
 * Pagination state that returns to the first page whenever the filters change.
 *
 * Without this, narrowing a filter while past page 1 leaves the grid requesting
 * an offset the filtered result set no longer reaches, so it renders its empty
 * state even though matches exist on page 1.
 *
 * The reset happens during render rather than in an effect: an effect would let
 * one query fire against the stale page first, which briefly shows that same
 * wrong empty state. Assigning state during render is React's documented way to
 * adjust state when an input changes, and re-renders before anything commits.
 */
export function useFilteredPagination(
  filters: unknown,
  pageSize = 25,
): [
  GridPaginationModel,
  React.Dispatch<React.SetStateAction<GridPaginationModel>>,
] {
  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize,
  });

  const filterKey = JSON.stringify(filters ?? null);
  const [prevFilterKey, setPrevFilterKey] = useState(filterKey);

  if (filterKey !== prevFilterKey) {
    setPrevFilterKey(filterKey);
    setPaginationModel((prev) =>
      prev.page === 0 ? prev : { ...prev, page: 0 },
    );
  }

  return [paginationModel, setPaginationModel];
}
