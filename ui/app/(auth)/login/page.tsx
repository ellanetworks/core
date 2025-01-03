"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation"
import {
    Box,
    Button,
    TextField,
    Typography,
    Alert,
    CircularProgress,
} from "@mui/material";
import { login } from "@/queries/auth";
import { useCookies } from "react-cookie"


const LoginPage = () => {
    const router = useRouter()
    const [cookies, setCookie, removeCookie] = useCookies(['user_token']);
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError(null);

        try {
            const result = await login(username, password);

            if (result?.token) {
                setCookie("user_token", result.token, {
                    sameSite: true,
                    secure: true,
                    expires: new Date(new Date().getTime() + 60 * 60 * 1000), // 1 hour expiry
                });

                router.push("/dashboard");
            } else {
                throw new Error("Invalid response: Token not found.");
            }
        } catch (err: any) {
            setError(err.message || "Login failed");
        } finally {
            setLoading(false);
        }
    };

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
                    label="Username"
                    variant="outlined"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
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
