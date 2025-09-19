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
import { createUser } from "@/queries/users";
import { getStatus } from "@/queries/status";
import { RoleID } from "@/types/types";

const InitializePage = () => {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [checkingInitialization, setCheckingInitialization] = useState(true);

  useEffect(() => {
    const checkInitialization = async () => {
      try {
        const status = await getStatus();
        if (status?.initialized) {
          router.push("/login");
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
      await createUser("", email, RoleID.Admin, password);
      router.push("/login");
    } catch (err) {
      const error = err as Error;
      setError(error.message);
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
            Initialize Ella Core
          </Typography>
          <Typography variant="body1" sx={{ marginBottom: 2 }}>
            Create the first user
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 1 }}>
              {error}
            </Alert>
          )}

          <TextField
            label="Email"
            type="email"
            variant="outlined"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            fullWidth
            margin="normal"
            required
          />
          <TextField
            label="Password"
            type="password"
            variant="outlined"
            margin="normal"
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
            sx={{ mt: 2 }}
            disabled={loading}
          >
            {loading ? <CircularProgress size={24} /> : "Create"}
          </Button>
        </form>
      </Box>
    </Box>
  );
};

export default InitializePage;
