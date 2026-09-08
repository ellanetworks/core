// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { Box, CssBaseline, Toolbar, AppBar } from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@/queries/queryClient";
import { Outlet } from "react-router-dom";
import theme from "@/utils/theme";
import Logo from "@/components/Logo";
import ProductTitle from "@/components/ProductTitle";

export default function AuthLayout() {
  const [queryClient] = useState(createQueryClient);

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider
        theme={theme}
        noSsr
        forceThemeRerender
        disableTransitionOnChange
      >
        <CssBaseline />
        <Box sx={{ display: "flex" }}>
          <AppBar position="fixed" sx={{ zIndex: (t) => t.zIndex.drawer + 1 }}>
            <Toolbar>
              <Logo width={50} height={50} />
              <ProductTitle />
            </Toolbar>
          </AppBar>

          <Box component="main" sx={{ flexGrow: 1, p: 3 }}>
            <Toolbar />
            <Outlet />
          </Box>
        </Box>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
