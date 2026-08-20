// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { createTheme } from "@mui/material/styles";
import type {} from "@mui/x-data-grid/themeAugmentation";

export interface ChartPalette {
  uplink: string;
  downlink: string;
  series: string[];
  protocols: Record<number, string>;
}

declare module "@mui/material/styles" {
  interface Palette {
    link: string;
    backgroundSubtle: string;
    chart: ChartPalette;
  }
  interface PaletteOptions {
    link?: string;
    backgroundSubtle?: string;
    chart?: ChartPalette;
  }
}

const base = createTheme({
  palette: {
    // MUI defaults to 3, which lets getContrastText return white on backgrounds
    // that only reach 3:1 — below the 4.5:1 WCAG 1.4.3 needs for chip-sized text.
    contrastThreshold: 4.5,
    primary: {
      main: "#26374a",
    },
    success: {
      main: "#1b6c1c",
    },
    error: {
      main: "#c62828",
    },
    warning: {
      main: "#ed6c02",
    },
    link: "#2B3FD4",
    backgroundSubtle: "#F5F5F5",
    chart: {
      uplink: "#FF9800",
      downlink: "#4254FB",
      series: [
        "#2196F3",
        "#4CAF50",
        "#FF9800",
        "#C2185B",
        "#9C27B0",
        "#00BCD4",
        "#FF5722",
        "#795548",
        "#546E7A",
        "#8BC34A",
        "#3F51B5",
        "#CDDC39",
      ],
      protocols: {
        1: "#FF9800",
        6: "#2196F3",
        17: "#4CAF50",
        47: "#9C27B0",
        58: "#C2185B",
        132: "#00BCD4",
      },
    },
  },
  components: {
    MuiDataGrid: {
      styleOverrides: {
        columnHeaderTitle: {
          fontWeight: 600,
        },
      },
    },
  },
  typography: {
    fontFamily: "Source Code Pro, monospace",
    fontWeightMedium: 500,
    fontWeightRegular: 500,
    body1: {
      fontWeight: 500,
    },
    h1: {
      fontWeight: 500,
    },
    h2: {
      fontWeight: 500,
    },
    h3: {
      fontWeight: 500,
    },
  },
});

const theme = createTheme(base, {
  palette: {
    DataGrid: { headerBg: base.palette.backgroundSubtle },
  },
  components: {
    MuiListItemText: {
      styleOverrides: {
        primary: { color: base.palette.primary.main },
      },
    },
  },
});

export default theme;
