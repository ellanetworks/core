import React, { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import { CssBaseline } from "@mui/material";
import { Outlet } from "react-router-dom";
import theme from "@/utils/theme";
import DrawerLayout from "@/components/DrawerLayout";
import { AuthProvider } from "@/contexts/AuthContext";

export default function CoreLayout() {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <AuthProvider>
          <DrawerLayout>
            <Outlet />
          </DrawerLayout>
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
