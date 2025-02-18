"use client";

import React, { createContext, useState, useEffect, ReactNode, useContext } from "react";
import { useCookies } from "react-cookie";
import { useRouter } from "next/navigation";
import { jwtDecode } from 'jwt-decode';
import { CircularProgress, Box } from "@mui/material";

interface AuthContextType {
    email: string | null;
    role: string | null; // "Admin" or "Read Only"
    setAuthData: (authData: { email: string; role: string } | null) => void;
}

export const AuthContext = createContext<AuthContextType>({
    email: null,
    role: null,
    setAuthData: () => { },
});

interface AuthProviderProps {
    children: ReactNode;
}

interface DecodedToken {
    email: string;
    role: number | string;
}

export const AuthProvider = ({ children }: AuthProviderProps) => {
    const [authData, setAuthData] = useState<{ email: string; role: string } | null>(null);
    const [cookies] = useCookies(["user_token"]);
    const router = useRouter();

    useEffect(() => {
        const token = cookies.user_token;
        if (!token) {
            router.push("/login");
            return;
        }

        try {
            const decoded = jwtDecode(token) as DecodedToken;

            let roleString = "";
            if (decoded.role === 0 || decoded.role === "0") {
                roleString = "Admin";
            } else if (decoded.role === 1 || decoded.role === "1") {
                roleString = "Read Only";
            } else {
                roleString = String(decoded.role);
            }
            setAuthData({ email: decoded.email, role: roleString });
            console.log("Auth Data set:", decoded.email, roleString);
        } catch (error) {
            console.error("Error decoding token", error);
            router.push("/login");
        }
    }, [cookies.user_token, router]);

    // While authData is not set, show a spinner.
    if (!authData) {
        return (
            <Box sx={{ display: "flex", justifyContent: "center", alignItems: "center", height: "100vh" }}>
                <CircularProgress />
            </Box>
        );
    }

    return (
        <AuthContext.Provider value={{ email: authData.email, role: authData.role, setAuthData }}>
            {children}
        </AuthContext.Provider>
    );
};

export const useAuth = () => useContext(AuthContext);
