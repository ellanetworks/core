"use client";

import React, { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import { CssBaseline } from "@mui/material";
import theme from "@/utils/theme";
import DrawerLayout from "@/components/DrawerLayout";
import { AuthProvider } from "@/contexts/AuthContext";
import useTokenValidation from "@/utils/token_validation";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());

  useTokenValidation();

  return (
    <html lang="en">
      <head>
        <title>Ella Core</title>
      </head>
      <body>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <AuthProvider>
              <DrawerLayout>{children}</DrawerLayout>
            </AuthProvider>
          </ThemeProvider>
        </QueryClientProvider>
      </body>
    </html>
  );
}
