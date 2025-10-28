"use client";

import React, {
  createContext,
  useState,
  useEffect,
  ReactNode,
  useContext,
  useRef,
  useCallback,
} from "react";
import { useRouter } from "next/navigation";
import { jwtDecode } from "jwt-decode";
import { CircularProgress, Box } from "@mui/material";
import { refresh } from "@/queries/auth";

type AuthState = { email: string; role: string };
interface AuthContextType {
  email: string | null;
  role: string | null;
  accessToken: string | null;
  authReady: boolean;
  setAuthData: React.Dispatch<React.SetStateAction<AuthState | null>>;
}

export const AuthContext = createContext<AuthContextType>({
  email: null,
  role: null,
  accessToken: null,
  authReady: false,
  setAuthData: () => {},
});

interface AuthProviderProps {
  children: ReactNode;
}
interface DecodedToken {
  email: string;
  role_id: number;
  exp?: number;
}

const LEEWAY_SEC = 120;
const MIN_REFRESH_DELAY_MS = 5000; // clamp to avoid 0ms loops

function roleToString(roleId: number): string {
  switch (roleId) {
    case 1:
      return "Admin";
    case 2:
      return "Read Only";
    case 3:
      return "Network Manager";
    default:
      return "Unknown";
  }
}

function tokenExpiringSoon(token: string, leewaySec = LEEWAY_SEC): boolean {
  try {
    const { exp } = jwtDecode<DecodedToken>(token);
    if (!exp) return true;
    const now = Math.floor(Date.now() / 1000);
    return exp - now <= leewaySec;
  } catch {
    return true;
  }
}

export const AuthProvider = ({ children }: AuthProviderProps) => {
  const [authData, setAuthData] = useState<AuthState | null>(null);
  const [authReady, setAuthReady] = useState(false);
  const [accessToken, setAccessToken] = useState<string | null>(null);

  const router = useRouter();
  const tokenRef = useRef<string | null>(null);
  const refreshTimerRef = useRef<number | null>(null);
  const refreshingRef = useRef(false);

  const clearRefreshTimer = useCallback(() => {
    if (refreshTimerRef.current != null) {
      window.clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }
  }, []);

  const scheduleRefresh = useCallback((token: string) => {
    clearRefreshTimer();
    let delayMs = 30_000; // default fallback
    try {
      const { exp } = jwtDecode<DecodedToken>(token);
      if (exp) {
        const now = Math.floor(Date.now() / 1000);
        delayMs = Math.max(
          MIN_REFRESH_DELAY_MS,
          (exp - LEEWAY_SEC - now) * 1000,
        );
      }
    } catch {
      // keep fallback
    }
    refreshTimerRef.current = window.setTimeout(() => {
      void silentRefresh();
    }, delayMs);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // intentionally stable (no deps)

  const silentRefresh = useCallback(async () => {
    if (refreshingRef.current) return;
    refreshingRef.current = true;
    try {
      const resp = await refresh();
      const token = (resp?.token as string) || "";
      if (!token) throw new Error("Missing token");

      tokenRef.current = token;
      setAccessToken(token);

      const decoded = jwtDecode<DecodedToken>(token);
      const role = roleToString(decoded.role_id);
      setAuthData({ email: decoded.email, role });

      if (tokenExpiringSoon(token)) {
        refreshTimerRef.current = window.setTimeout(() => {
          void silentRefresh();
        }, MIN_REFRESH_DELAY_MS);
      } else {
        scheduleRefresh(token);
      }
    } catch {
      tokenRef.current = null;
      setAccessToken(null);
      setAuthData(null);
      clearRefreshTimer();
      router.push("/login");
    } finally {
      refreshingRef.current = false;
      setAuthReady(true);
    }
  }, [router, scheduleRefresh, clearRefreshTimer]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await silentRefresh();
      } finally {
        if (!cancelled) setAuthReady(true);
      }
    })();
    return () => {
      cancelled = true;
      clearRefreshTimer();
    };
  }, [silentRefresh, clearRefreshTimer]);

  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState !== "visible") return;
      const t = tokenRef.current;
      if (!t || tokenExpiringSoon(t)) {
        void silentRefresh();
      }
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, [silentRefresh]);

  if (!authReady) {
    return (
      <Box
        sx={{
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          height: "100vh",
        }}
      >
        <CircularProgress />
      </Box>
    );
  }

  return (
    <AuthContext.Provider
      value={{
        email: authData?.email ?? null,
        role: authData?.role ?? null,
        accessToken,
        authReady,
        setAuthData,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);
