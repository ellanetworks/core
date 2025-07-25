"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import { lookupToken } from "@/queries/auth";

const useTokenValidation = () => {
  const router = useRouter();
  const [cookies, , removeCookie] = useCookies(["user_token"]);

  useEffect(() => {
    const checkToken = async () => {
      try {
        const response = await lookupToken(cookies.user_token);
        if (!response.valid) {
          removeCookie("user_token");
          router.push("/login");
        }
      } catch (error) {
        console.error("Token validation failed:", error);
        removeCookie("user_token");
        router.push("/login");
      }
    };

    if (cookies.user_token) {
      checkToken();
    } else {
      router.push("/login");
    }
  }, [cookies.user_token, router, removeCookie]);
};

export default useTokenValidation;
