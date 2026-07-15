// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { createTheme } from "@mui/material/styles";
import type {} from "@mui/x-data-grid/themeAugmentation";

declare module "@mui/material/styles" {
  interface Palette {
    link: string;
    backgroundSubtle: string;
  }
  interface PaletteOptions {
    link?: string;
    backgroundSubtle?: string;
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
  },
  components: {
    MuiListItemText: {
      styleOverrides: {
        primary: {
          color: "#26374a",
        },
      },
    },
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

// A second pass so the grid header can reference a token from the first.
const theme = createTheme(base, {
  palette: {
    DataGrid: { headerBg: base.palette.backgroundSubtle },
  },
});

export default theme;
