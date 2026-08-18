// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import type { GridPaginationModel } from "@mui/x-data-grid";

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
