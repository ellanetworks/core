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

const theme = createTheme({
  palette: {
    primary: {
      main: "#26374a",
    },
    success: {
      main: "#1b6c1c",
    },
    error: {
      main: "#c62828",
    },
    link: "#4254FB",
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

export default theme;
