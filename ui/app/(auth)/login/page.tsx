"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Box,
  Button,
  TextField,
  Typography,
  Alert,
  CircularProgress,
} from "@mui/material";
import { login } from "@/queries/auth";
import { useCookies } from "react-cookie";
import { getStatus } from "@/queries/status";

const LoginPage = () => {
  const router = useRouter();
  const [, setCookie,] = useCookies(["user_token"]);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [checkingInitialization, setCheckingInitialization] = useState(true);

  useEffect(() => {
    const checkInitialization = async () => {
      try {
        const status = await getStatus();
        if (!status?.initialized) {
          router.push("/initialize");
        } else {
          setCheckingInitialization(false);
        }
      } catch (err) {
        console.error("Failed to fetch system status:", err);
        setError("Failed to check system initialization.");
      }
    };

    checkInitialization();
  }, [router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const result = await login(email, password);

      if (result?.token) {
        setCookie("user_token", result.token, {
          sameSite: true,
          expires: new Date(new Date().getTime() + 60 * 60 * 1000), // 1 hour expiry
        });

        router.push("/dashboard");
      } else {
        throw new Error("Invalid response: Token not found.");
      }
    } catch (err) {
      const error = err as Error;
      setError(error.message || "Login failed");
    } finally {
      setLoading(false);
    }
  };

  if (checkingInitialization) {
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
        height: "100vh",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        padding: 2,
      }}
    >
      <Box
        component="form"
        onSubmit={handleSubmit}
        sx={{
          width: 300,
          display: "flex",
          flexDirection: "column",
          gap: 2,
          border: "1px solid",
          borderColor: "divider",
          borderRadius: 2,
          padding: 3,
          boxShadow: 2,
        }}
      >
        <Typography variant="h5" textAlign="center">
          Login
        </Typography>

        {error && <Alert severity="error">{error}</Alert>}

        <TextField
          label="Email"
          variant="outlined"
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
          onChange={(e) => setPassword(e.target.value)}
          fullWidth
          required
        />

        <Button
          type="submit"
          variant="contained"
          color="success"
          fullWidth
          disabled={loading}
        >
          {loading ? <CircularProgress size={24} /> : "Login"}
        </Button>
      </Box>
    </Box>
  );
};

export default LoginPage;
