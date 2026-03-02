import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Button,
  TextField,
  Typography,
  CircularProgress,
} from "@mui/material";
import { login, refresh } from "@/queries/auth";
import { getStatus } from "@/queries/status";
import { useSnackbar } from "@/contexts/SnackbarContext";

const LoginPage = () => {
  const navigate = useNavigate();
  const { showSnackbar } = useSnackbar();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

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

  useEffect(() => {
    if (checkingInitialization) return;
    (async () => {
      try {
        const r = await refresh();
        if (r?.token) {
          navigate("/dashboard", { state: { token: r.token } });
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

      navigate("/dashboard", { state: { token: loginResp.token } });
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
          <Typography variant="h5" textAlign="center" gutterBottom>
            Login
          </Typography>

          <TextField
            label="Email"
            type="email"
            variant="outlined"
            margin="normal"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            fullWidth
            required
          />

          <TextField
            label="Password"
            type="password"
            variant="outlined"
            value={password}
            margin="normal"
            onChange={(e) => setPassword(e.target.value)}
            fullWidth
            required
          />

          <Button
            type="submit"
            variant="contained"
            color="success"
            fullWidth
            sx={{ mt: 2 }}
            disabled={loading}
          >
            {loading ? <CircularProgress size={24} /> : "Login"}
          </Button>
        </form>
      </Box>
    </Box>
  );
};

export default LoginPage;
