// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { createTheme } from "@mui/material/styles";
import type {} from "@mui/x-data-grid/themeAugmentation";
import { dark, light, type Tokens } from "@/utils/tokens";

export interface ChartPalette {
  uplink: string;
  downlink: string;
  series: string[];
  protocols: Record<number, string>;
}

declare module "@mui/material/styles" {
  interface CssThemeVariables {
    enabled: true;
  }
  interface Palette {
    link: string;
    borderControl: string;
    backgroundSubtle: string;
    chart: ChartPalette;
  }
  interface PaletteOptions {
    link?: string;
    borderControl?: string;
    backgroundSubtle?: string;
    chart?: ChartPalette;
  }
}

const paletteFor = (tokens: Tokens) => ({
  // MUI defaults to 3, which lets getContrastText return white on backgrounds
  // that only reach 3:1 — below the 4.5:1 WCAG 1.4.3 needs for chip-sized text.
  contrastThreshold: 4.5,
  primary: { main: tokens.primary },
  success: { main: tokens.success },
  error: { main: tokens.error },
  warning: { main: tokens.warning },
  info: { main: tokens.info },
  link: tokens.link,
  background: {
    default: tokens.backgroundDefault,
    paper: tokens.backgroundPaper,
  },
  text: {
    primary: tokens.textPrimary,
    secondary: tokens.textSecondary,
  },
  backgroundSubtle: tokens.backgroundSubtle,
  borderControl: tokens.borderControl,
  chart: tokens.chart,
  DataGrid: { headerBg: tokens.backgroundSubtle },
});

const theme = createTheme({
  cssVariables: { colorSchemeSelector: "class" },
  colorSchemes: {
    light: { palette: { mode: "light", ...paletteFor(light) } },
    dark: { palette: { mode: "dark", ...paletteFor(dark) } },
  },
  components: {
    MuiDataGrid: {
      styleOverrides: {
        columnHeaderTitle: {
          fontWeight: 600,
        },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        notchedOutline: ({ theme: t }) => ({
          borderColor: t.vars.palette.borderControl,
        }),
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

export const THEME_PROVIDER_PROPS = {
  noSsr: true,
  forceThemeRerender: true,
  disableTransitionOnChange: true,
} as const;

export default theme;
