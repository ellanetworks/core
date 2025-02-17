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
            await createUser("", email, 0, password);
            router.push("/login");
        } catch (err: any) {
            setError(err.message || "User creation failed");
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
                <Typography variant="h5">
                    Initialize Ella Core
                </Typography>
                <Typography variant="body1" sx={{ marginBottom: 2 }}>
                    Create the first user
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
                    {loading ? <CircularProgress size={24} /> : "Create"}
                </Button>
            </Box>
        </Box>
    );
};

export default InitializePage;
