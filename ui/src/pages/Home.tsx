import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { getStatus } from "@/queries/status";

export default function Home() {
  const navigate = useNavigate();

  useEffect(() => {
    const checkInitialization = async () => {
      try {
        const status = await getStatus();
        if (status?.initialized) {
          navigate("/login", { replace: true });
        } else {
          navigate("/initialize", { replace: true });
        }
      } catch (err) {
        console.error("Failed to check Ella initialization status:", err);
      }
    };

    checkInitialization();
  }, [navigate]);

  return null;
}
