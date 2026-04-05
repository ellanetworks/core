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
import RadiosListTab from "./pages/radios/RadiosListTab";
import RadiosEventsTab from "./pages/radios/EventsTab";
import RadioDetail from "./pages/RadioDetail";
import Profiles from "./pages/Profiles";
import ProfileDetail from "./pages/ProfileDetail";
import PolicyDetail from "./pages/PolicyDetail";
import Networking from "./pages/Networking";
import DataNetworksTab from "./pages/networking/DataNetworksTab";
import SlicesTab from "./pages/networking/SlicesTab";
import InterfacesTab from "./pages/networking/InterfacesTab";
import RoutesTab from "./pages/networking/RoutesTab";
import NATTab from "./pages/networking/NATTab";
import BGPTab from "./pages/networking/BGPTab";
import FlowAccountingTab from "./pages/networking/FlowAccountingTab";
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
        {/* Note: a radio named "events" would match the nested route below
            instead of radios/:name. Radio names are system-generated so this
            collision cannot occur in practice. */}
        <Route path="radios" element={<Radios />}>
          <Route index element={<RadiosListTab />} />
          <Route path="events" element={<RadiosEventsTab />} />
        </Route>
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
        <Route path="networking" element={<Networking />}>
          <Route index element={<Navigate to="data-networks" replace />} />
          <Route path="data-networks" element={<DataNetworksTab />} />
          <Route path="slices" element={<SlicesTab />} />
          <Route path="interfaces" element={<InterfacesTab />} />
          <Route path="routes" element={<RoutesTab />} />
          <Route path="nat" element={<NATTab />} />
          <Route path="bgp" element={<BGPTab />} />
          <Route path="flow-accounting" element={<FlowAccountingTab />} />
        </Route>
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
