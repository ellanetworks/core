// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useLocation, useSearchParams } from "react-router-dom";
import {
  Alert,
  Box,
  Button,
  Divider,
  IconButton,
  InputAdornment,
  TextField,
  CircularProgress,
} from "@mui/material";
import { Visibility, VisibilityOff } from "@mui/icons-material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { login, refresh } from "@/queries/auth";
import {
  getOIDCLogin,
  OIDC_LOGIN_PATH,
  oidcLoginError,
  type OIDCLogin,
} from "@/queries/oidc";
import { getStatus } from "@/queries/status";
import { useSnackbar } from "@/contexts/SnackbarContext";
import PageTitle from "@/components/PageTitle";
import {
  forgetReturnPath,
  rememberReturnPath,
  returnPath,
} from "@/utils/returnPath";

const schema = yup.object().shape({
  email: yup
    .string()
    .email("Must be a valid email")
    .required("Email is required"),
  password: yup
    .string()
    .min(1, "Password is required")
    .required("Password is required"),
});

const LoginPage = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [redirectTo] = useState(
    () =>
      (location.state as { from?: string } | null)?.from ||
      returnPath() ||
      "/dashboard",
  );
  const [params, setParams] = useSearchParams();
  const [oidcError] = useState(() => params.get("error"));
  const { showSnackbar } = useSnackbar();

  useEffect(() => {
    if (params.has("error")) {
      setParams(
        (current) => {
          const next = new URLSearchParams(current);
          next.delete("error");
          return next;
        },
        { replace: true, state: location.state },
      );
    }
  }, [params, setParams, location.state]);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);

  const [checkingInitialization, setCheckingInitialization] = useState(true);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [oidc, setOIDC] = useState<OIDCLogin>({ enabled: false });
  const ssoButton = useRef<HTMLAnchorElement>(null);

  useEffect(() => {
    if (!checkingAuth && oidc.enabled) ssoButton.current?.focus();
  }, [checkingAuth, oidc.enabled]);

  useEffect(() => {
    (async () => {
      try {
        const status = await getStatus();
        if (!status?.initialized) {
          navigate("/initialize");
          return;
        }
        setCheckingInitialization(false);
      } catch (err) {
        console.error("Failed to fetch system status:", err);
        showSnackbar("Failed to check system initialization.", "error");
        setCheckingInitialization(false);
      }
    })();
  }, [navigate, showSnackbar]);

  const validateField = async (field: string, value: string) => {
    try {
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
      await fieldSchema.validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
    } catch (err) {
      if (err instanceof ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const isValid = useMemo(() => {
    try {
      schema.validateSync({ email, password }, { abortEarly: false });
      return true;
    } catch {
      return false;
    }
  }, [email, password]);

  useEffect(() => {
    if (checkingInitialization) return;
    (async () => {
      const oidcLogin = getOIDCLogin().catch((): OIDCLogin => ({
        enabled: false,
      }));
      try {
        const r = await refresh();
        if (r?.token) {
          forgetReturnPath();
          navigate(redirectTo, { replace: true });
          return;
        }
      } catch {}
      setOIDC(await oidcLogin);
      setCheckingAuth(false);
    })();
  }, [checkingInitialization, navigate, redirectTo]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      const loginResp = await login(email, password);

      if (!loginResp?.token)
        throw new Error("Login succeeded but could not obtain access token.");

      forgetReturnPath();
      navigate(redirectTo);
    } catch (err) {
      const error = err as Error;
      showSnackbar(error.message || "Login failed", "error");
    } finally {
      setLoading(false);
    }
  };

  if (checkingInitialization || checkingAuth) {
    return (
      <Box
        sx={{
          flexGrow: 1,
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        flexGrow: 1,
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        p: 2,
      }}
    >
      <Box
        sx={{
          width: "100%",
          maxWidth: 360,
          display: "flex",
          flexDirection: "column",
          gap: 2,
          border: "1px solid",
          borderColor: "divider",
          borderRadius: 2,
          p: 3,
          boxShadow: 2,
        }}
      >
        <form onSubmit={handleSubmit} noValidate>
          <PageTitle
            title="Login"
            size="h5"
            gutterBottom
            sx={{ textAlign: "center" }}
          />

          {oidcError && (
            <Alert severity="error" sx={{ mt: 1 }}>
              {oidcLoginError(oidcError)}
            </Alert>
          )}
          {oidc.enabled && (
            <>
              <Button
                variant="contained"
                color="success"
                fullWidth
                disableFocusRipple
                sx={{
                  mt: 2,
                  "&.Mui-focusVisible": {
                    outline: "2px solid",
                    outlineColor: "success.main",
                    outlineOffset: 2,
                  },
                }}
                href={OIDC_LOGIN_PATH}
                onClick={() => rememberReturnPath(redirectTo)}
                ref={ssoButton}
              >
                Log in with {oidc.display_name}
              </Button>
              <Divider sx={{ mt: 2 }}>or</Divider>
            </>
          )}

          <TextField
            label="Email"
            type="email"
            variant="outlined"
            margin="normal"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              validateField("email", e.target.value);
            }}
            onBlur={() => handleBlur("email")}
            error={!!errors.email && touched.email}
            helperText={touched.email ? errors.email : ""}
            fullWidth
            required
            autoFocus={!oidc.enabled}
            autoComplete="email"
          />

          <TextField
            label="Password"
            type={showPassword ? "text" : "password"}
            variant="outlined"
            value={password}
            margin="normal"
            onChange={(e) => {
              setPassword(e.target.value);
              validateField("password", e.target.value);
            }}
            onBlur={() => handleBlur("password")}
            error={!!errors.password && touched.password}
            helperText={touched.password ? errors.password : ""}
            fullWidth
            required
            autoComplete="current-password"
            slotProps={{
              input: {
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      aria-label={
                        showPassword ? "Hide password" : "Show password"
                      }
                      onClick={() => setShowPassword((prev) => !prev)}
                      edge="end"
                    >
                      {showPassword ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              },
            }}
          />

          <Button
            type="submit"
            variant={oidc.enabled ? "outlined" : "contained"}
            color="success"
            fullWidth
            sx={{ mt: 2 }}
            disabled={!isValid || loading}
          >
            {loading ? <CircularProgress size={24} /> : "Login"}
          </Button>
        </form>
      </Box>
    </Box>
  );
};

export default LoginPage;
