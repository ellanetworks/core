import { Routes, Route, Navigate } from "react-router-dom";
import AuthLayout from "./layouts/AuthLayout";
import CoreLayout from "./layouts/CoreLayout";
import Home from "./pages/Home";
import Login from "./pages/Login";
import Initialize from "./pages/Initialize";
import Dashboard from "./pages/Dashboard";
import Subscribers from "./pages/Subscribers";
import Radios from "./pages/Radios";
import Policies from "./pages/Policies";
import Networking from "./pages/Networking";
import Operator from "./pages/Operator";
import Users from "./pages/Users";
import Profile from "./pages/Profile";
import AuditLogs from "./pages/AuditLogs";
import BackupRestore from "./pages/BackupRestore";
import Traffic from "./pages/Traffic";

export default function AppRouter() {
  return (
    <Routes>
      <Route index element={<Home />} />

      <Route element={<AuthLayout />}>
        <Route path="login" element={<Login />} />
        <Route path="initialize" element={<Initialize />} />
      </Route>

      <Route element={<CoreLayout />}>
        <Route path="dashboard" element={<Dashboard />} />
        <Route path="subscribers" element={<Subscribers />} />
        <Route path="radios" element={<Radios />} />
        <Route path="policies" element={<Policies />} />
        <Route path="networking" element={<Networking />} />
        <Route path="operator" element={<Operator />} />
        <Route path="users" element={<Users />} />
        <Route path="profile" element={<Profile />} />
        <Route path="audit-logs" element={<AuditLogs />} />
        <Route path="backup-restore" element={<BackupRestore />} />
        <Route path="traffic/usage" element={<Traffic />} />
        <Route path="traffic/flows" element={<Traffic />} />
        <Route
          path="traffic"
          element={<Navigate to="/traffic/usage" replace />}
        />
        <Route
          path="usage"
          element={<Navigate to="/traffic/usage" replace />}
        />
        <Route
          path="flow-reports"
          element={<Navigate to="/traffic/flows" replace />}
        />
      </Route>

      {/* Catch-all redirect */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
