// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, {
  useEffect,
  useState,
  type ReactElement,
  type ReactNode,
} from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import {
  MemoryRouter,
  useLocation,
  useNavigate,
  type InitialEntry,
} from "react-router-dom";
import theme from "@/utils/theme";
import { AuthContext } from "@/contexts/AuthContext";
import { SnackbarProvider } from "@/contexts/SnackbarContext";
import { setOnUnauthorized } from "@/queries/utils";

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

export const LOCATION_TEST_ID = "test-location";

const LocationProbe = () => {
  const { pathname } = useLocation();
  return (
    <span data-testid={LOCATION_TEST_ID} style={{ display: "none" }}>
      {pathname}
    </span>
  );
};

const TestAuthProvider = ({
  auth,
  children,
}: {
  auth: TestAuth;
  children: ReactNode;
}) => {
  const navigate = useNavigate();
  const [accessToken, setAccessToken] = useState<string | null>(
    auth.accessToken ?? "test-token",
  );
  const [signedOut, setSignedOut] = useState(false);

  useEffect(() => {
    setOnUnauthorized(() => {
      setAccessToken(null);
      setSignedOut(true);
      navigate("/login");
    });
    return () => setOnUnauthorized(null);
  }, [navigate]);

  return (
    <AuthContext.Provider
      value={{
        email: signedOut ? null : (auth.email ?? "admin@ellanetworks.com"),
        role: signedOut ? null : (auth.role ?? "Admin"),
        accessToken,
        authReady: auth.authReady ?? true,
        setAuthData: () => {},
      }}
    >
      <SnackbarProvider>{signedOut ? null : children}</SnackbarProvider>
    </AuthContext.Provider>
  );
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

  const Wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <ThemeProvider theme={theme}>
        <MemoryRouter initialEntries={initialEntries}>
          {auth ? (
            <>
              <LocationProbe />
              <TestAuthProvider auth={auth}>{children}</TestAuthProvider>
            </>
          ) : (
            children
          )}
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );

  return { client, ...render(ui, { wrapper: Wrapper, ...options }) };
}
