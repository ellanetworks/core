// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import {
  DataGrid,
  type DataGridProps,
  type GridPaginationModel,
  type GridValidRowModel,
} from "@mui/x-data-grid";

export const EMBEDDED_GRID_HEIGHT = 421;

export const LIST_PAGE_SIZE_OPTIONS = [10, 25, 50, 100];

export const EMBEDDED_PAGE_SIZE_OPTIONS = [10, 25, 50];

const PREFERRED_PAGE_SIZE = 25;

export interface EntityGridProps<R extends GridValidRowModel> extends Omit<
  DataGridProps<R>,
  | "disableColumnMenu"
  | "disableRowSelectionOnClick"
  | "pageSizeOptions"
  | "autoHeight"
> {
  variant?: "list" | "log" | "embedded";
  height?: number | string;
  pageSizeOptions?: number[];
  defaultPageSize?: number;
}

const listSx = {
  width: "100%",
  border: 1,
  borderColor: "divider",
  "& .MuiDataGrid-cell": {
    borderBottom: "1px solid",
    borderColor: "divider",
  },
  "& .MuiDataGrid-columnHeaders": {
    borderBottom: "1px solid",
    borderColor: "divider",
  },
  "& .MuiDataGrid-footerContainer": {
    borderTop: "1px solid",
    borderColor: "divider",
  },
};

const embeddedSx = {
  border: 1,
  borderColor: "divider",
  "& .MuiDataGrid-cell": {
    borderBottom: "1px solid",
    borderColor: "divider",
  },
};

const resolvePageSize = (
  defaultPageSize: number | undefined,
  options: number[],
) => {
  if (defaultPageSize !== undefined) {
    return defaultPageSize;
  }
  if (options.includes(PREFERRED_PAGE_SIZE)) {
    return PREFERRED_PAGE_SIZE;
  }
  return options[0] ?? PREFERRED_PAGE_SIZE;
};

export default function EntityGrid<R extends GridValidRowModel>({
  variant = "list",
  height,
  defaultPageSize,
  pageSizeOptions,
  paginationModel,
  onPaginationModelChange,
  density,
  sx,
  ...rest
}: EntityGridProps<R>) {
  const options =
    pageSizeOptions ??
    (variant === "embedded"
      ? EMBEDDED_PAGE_SIZE_OPTIONS
      : LIST_PAGE_SIZE_OPTIONS);

  const [fallbackModel, setFallbackModel] = useState<GridPaginationModel>(
    () => ({
      page: 0,
      pageSize: resolvePageSize(defaultPageSize, options),
    }),
  );

  return (
    <DataGrid<R>
      {...rest}
      density={
        density ??
        (variant === "embedded" || variant === "log" ? "compact" : undefined)
      }
      pageSizeOptions={options}
      paginationModel={paginationModel ?? fallbackModel}
      onPaginationModelChange={onPaginationModelChange ?? setFallbackModel}
      disableColumnMenu
      disableRowSelectionOnClick
      sx={[
        variant === "embedded" ? embeddedSx : listSx,
        height !== undefined ? { height } : false,
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
    />
  );
}
