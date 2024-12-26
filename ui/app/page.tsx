"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { getStatus } from "@/queries/status";

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    const checkInitialization = async () => {
      try {
        const status = await getStatus();
        if (status?.initialized) {
          router.replace("/login");
        } else {
          router.replace("/initialize");
        }
      } catch (err) {
        console.error("Failed to check Ella initialization status:", err);
      }
    };

    checkInitialization();
  }, [router]);

  return null;
}
