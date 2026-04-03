import { Routes, Route, Navigate } from "react-router-dom";
import AuthLayout from "./layouts/AuthLayout";
import CoreLayout from "./layouts/CoreLayout";
import Home from "./pages/Home";
import Login from "./pages/Login";
import Initialize from "./pages/Initialize";
import Dashboard from "./pages/Dashboard";
import Subscribers from "./pages/Subscribers";
import SubscriberDetail from "./pages/SubscriberDetail";
import Radios from "./pages/Radios";
import RadioDetail from "./pages/RadioDetail";
import Profiles from "./pages/Profiles";
import ProfileDetail from "./pages/ProfileDetail";
import PolicyDetail from "./pages/PolicyDetail";
import Networking from "./pages/Networking";
import DataNetworkDetail from "./pages/DataNetworkDetail";
import Operator from "./pages/Operator";
import Users from "./pages/Users";
import UserDetail from "./pages/UserDetail";
import Profile from "./pages/Profile";
import AuditLogs from "./pages/AuditLogs";
import BackupRestore from "./pages/BackupRestore";
import Traffic from "./pages/Traffic";
import RequireAdmin from "./components/RequireAdmin";

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
        <Route path="subscribers/:imsi" element={<SubscriberDetail />} />
        <Route path="radios" element={<Radios />} />
        <Route path="radios/:name" element={<RadioDetail />} />
        <Route path="profiles" element={<Profiles />} />
        <Route path="profiles/:name" element={<ProfileDetail />} />
        <Route
          path="profiles/:profileName/policies/:policyName"
          element={<PolicyDetail />}
        />
        <Route path="policies" element={<Navigate to="/profiles" replace />} />
        <Route
          path="policies/:name"
          element={<Navigate to="/profiles" replace />}
        />
        <Route path="networking" element={<Networking />} />
        <Route
          path="networking/data-networks/:name"
          element={<DataNetworkDetail />}
        />
        <Route path="operator" element={<Operator />} />
        <Route element={<RequireAdmin />}>
          <Route path="users" element={<Users />} />
          <Route path="users/:email" element={<UserDetail />} />
          <Route path="audit-logs" element={<AuditLogs />} />
          <Route path="backup-restore" element={<BackupRestore />} />
        </Route>
        <Route path="profile" element={<Profile />} />
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
