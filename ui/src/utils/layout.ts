// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const MAX_WIDTH = 1400;

export const PAGE_PADDING_X = { xs: 2, sm: 3, md: 4, lg: 5 };

export const DENSE_ROW_HEIGHT = 33;

export const DENSE_HEADER_HEIGHT = 39;

export const TABLE_CONTAINER_SX = {
  border: 1,
  borderColor: "divider",
  borderRadius: 1,
  bgcolor: "background.paper",
} as const;

export const SETTING_CELL_SX = { fontWeight: 600, width: "35%" } as const;

export const VALUE_CELL_SX = { width: "55%" } as const;

export const ACTION_CELL_SX = { width: "10%", textAlign: "right" } as const;

export const MIN_PANEL_WIDTH = 440;

type MeasurableColumn = { width?: number; minWidth?: number };

export const columnsMinWidth = (columns: MeasurableColumn[]) =>
  columns.reduce((total, col) => total + (col.minWidth ?? col.width ?? 100), 0);

export const splitGridColumns = (...panelMinWidths: number[]) => {
  const min = Math.max(MIN_PANEL_WIDTH, ...panelMinWidths);
  return `repeat(auto-fit, minmax(min(${min}px, 100%), 1fr))`;
};
