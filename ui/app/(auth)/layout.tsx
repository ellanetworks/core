"use client";

import React, { useState } from "react";
import {
  Box,
  CssBaseline,
  Toolbar,
  AppBar,
  Typography,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import theme from "@/utils/theme";
import "../globals.scss";
import Logo from "@/components/Logo";


export default function RootLayout({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <html lang="en">
      <head>
        <title>Ella</title>
        <link
          rel="shortcut icon"
          href="https://assets.ubuntu.com/v1/49a1a858-favicon-32x32.png"
          type="image/x-icon"
        />
      </head>
      <body>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <Box sx={{ display: "flex" }}>
              <AppBar
                position="fixed"
                sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}
              >
                <Toolbar>
                  <Logo width={50} height={50} />
                  <Typography variant="h6" noWrap component="div" sx={{ ml: 2 }}>
                    Ella Core
                  </Typography>
                </Toolbar>
              </AppBar>

              <Box component="main" sx={{ flexGrow: 1, p: 3 }}>
                <Toolbar />
                {children}
              </Box>
            </Box>
          </ThemeProvider>
        </QueryClientProvider>
      </body>
    </html>
  );
}