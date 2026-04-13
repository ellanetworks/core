import React, { useCallback, useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import {
  Box,
  Button,
  IconButton,
  InputAdornment,
  TextField,
  Typography,
  CircularProgress,
} from "@mui/material";
import { Visibility, VisibilityOff } from "@mui/icons-material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { login, refresh } from "@/queries/auth";
import { getStatus } from "@/queries/status";
import { useSnackbar } from "@/contexts/SnackbarContext";

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
  const redirectTo =
    (location.state as { from?: string } | null)?.from || "/dashboard";
  const { showSnackbar } = useSnackbar();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);

  const [checkingInitialization, setCheckingInitialization] = useState(true);
  const [checkingAuth, setCheckingAuth] = useState(true);

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
  }, [navigate]);

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

  const validateForm = useCallback(async () => {
    try {
      await schema.validate({ email, password }, { abortEarly: false });
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  }, [email, password]);

  useEffect(() => {
    validateForm();
  }, [email, password, validateForm]);

  useEffect(() => {
    if (checkingInitialization) return;
    (async () => {
      try {
        const r = await refresh();
        if (r?.token) {
          navigate(redirectTo, { state: { token: r.token } });
          return;
        }
      } catch {
      } finally {
        setCheckingAuth(false);
      }
    })();
  }, [checkingInitialization, navigate]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      const loginResp = await login(email, password);

      if (!loginResp?.token)
        throw new Error("Login succeeded but could not obtain access token.");

      navigate(redirectTo, { state: { token: loginResp.token } });
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
          height: "100vh",
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
        minHeight: "100vh",
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
          <Typography variant="h5" gutterBottom sx={{ textAlign: "center" }}>
            Login
          </Typography>

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
            autoFocus
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
            variant="contained"
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
