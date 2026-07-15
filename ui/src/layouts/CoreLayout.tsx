// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@/queries/queryClient";
import { ThemeProvider } from "@mui/material/styles";
import { CssBaseline } from "@mui/material";
import { Outlet } from "react-router-dom";
import theme from "@/utils/theme";
import DrawerLayout from "@/components/DrawerLayout";
import ErrorBoundary from "@/components/ErrorBoundary";
import { AuthProvider } from "@/contexts/AuthContext";

export default function CoreLayout() {
  const [queryClient] = useState(createQueryClient);

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <AuthProvider>
          <DrawerLayout>
            <ErrorBoundary>
              <Outlet />
            </ErrorBoundary>
          </DrawerLayout>
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
