import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

export default function RequireAdmin() {
  const { role, accessToken } = useAuth();
  if (!accessToken) {
    return <Navigate to="/login" replace />;
  }
  if (role !== "Admin") {
    return <Navigate to="/dashboard" replace />;
  }
  return <Outlet />;
}
