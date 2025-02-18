import React from "react";
import { useAuth } from "@/contexts/AuthContext";

interface AdminOnlyProps {
    children: React.ReactNode;
}

const AdminOnly: React.FC<AdminOnlyProps> = ({ children }) => {
    const { role } = useAuth();
    // Log role
    console.log("Role is:", role);
    if (role !== "Admin") return null;
    return <>{children}</>;
};

export default AdminOnly;
