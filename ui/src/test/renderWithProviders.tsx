// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { type ReactElement, type ReactNode } from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import { MemoryRouter, type InitialEntry } from "react-router-dom";
import theme from "@/utils/theme";

export const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });

interface ProviderOptions extends Omit<RenderOptions, "wrapper"> {
  initialEntries?: InitialEntry[];
  queryClient?: QueryClient;
}

export function renderWithProviders(
  ui: ReactElement,
  { initialEntries = ["/"], queryClient, ...options }: ProviderOptions = {},
) {
  const client = queryClient ?? createTestQueryClient();

  const Wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <ThemeProvider theme={theme}>
        <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );

  return { client, ...render(ui, { wrapper: Wrapper, ...options }) };
}
