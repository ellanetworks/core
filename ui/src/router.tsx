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
import Usage from "./pages/Usage";
import FlowReports from "./pages/FlowReports";

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
        <Route path="usage" element={<Usage />} />
        <Route path="flow-reports" element={<FlowReports />} />
      </Route>

      {/* Catch-all redirect */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
