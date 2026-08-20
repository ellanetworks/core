// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { type ReactElement, type ReactNode } from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import { MemoryRouter, type InitialEntry } from "react-router-dom";
import theme from "@/utils/theme";
import { AuthContext } from "@/contexts/AuthContext";
import { SnackbarProvider } from "@/contexts/SnackbarContext";

export const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });

export type TestAuth = {
  email?: string;
  role?: string;
  accessToken?: string;
  authReady?: boolean;
};

interface ProviderOptions extends Omit<RenderOptions, "wrapper"> {
  initialEntries?: InitialEntry[];
  queryClient?: QueryClient;
  auth?: TestAuth;
}

export function renderWithProviders(
  ui: ReactElement,
  {
    initialEntries = ["/"],
    queryClient,
    auth,
    ...options
  }: ProviderOptions = {},
) {
  const client = queryClient ?? createTestQueryClient();

  const Wrapper = ({ children }: { children: ReactNode }) => {
    const inner = auth ? (
      <AuthContext.Provider
        value={{
          email: auth.email ?? "admin@ellanetworks.com",
          role: auth.role ?? "Admin",
          accessToken: auth.accessToken ?? "test-token",
          authReady: auth.authReady ?? true,
          setAuthData: () => {},
        }}
      >
        <SnackbarProvider>{children}</SnackbarProvider>
      </AuthContext.Provider>
    ) : (
      children
    );

    return (
      <QueryClientProvider client={client}>
        <ThemeProvider theme={theme}>
          <MemoryRouter initialEntries={initialEntries}>{inner}</MemoryRouter>
        </ThemeProvider>
      </QueryClientProvider>
    );
  };

  return { client, ...render(ui, { wrapper: Wrapper, ...options }) };
}
