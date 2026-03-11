import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

export default function RequireAdmin() {
  const { role } = useAuth();
  if (role !== "Admin") {
    return <Navigate to="/dashboard" replace />;
  }
  return <Outlet />;
}
