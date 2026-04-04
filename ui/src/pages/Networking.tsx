import { useMemo } from "react";
import type { SyntheticEvent } from "react";
import { Box, Typography, Tabs, Tab } from "@mui/material";
import { useTheme, createTheme } from "@mui/material/styles";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const TAB_SEGMENTS = [
  "data-networks",
  "slices",
  "interfaces",
  "routes",
  "nat",
  "bgp",
  "flow-accounting",
] as const;

type TabKey = (typeof TAB_SEGMENTS)[number];

export default function NetworkingPage() {
  const { role, accessToken } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";
  const { showSnackbar } = useSnackbar();

  const location = useLocation();
  const navigate = useNavigate();

  const match = location.pathname.match(/^\/networking\/([^/]+)/);
  const segment = match?.[1] as TabKey | undefined;
  const activeTab: TabKey =
    segment && (TAB_SEGMENTS as readonly string[]).includes(segment)
      ? segment
      : "data-networks";

  const outerTheme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(outerTheme, {
        palette: {
          DataGrid: { headerBg: outerTheme.palette.backgroundSubtle },
        },
      }),
    [outerTheme],
  );

  const handleTabChange = (_: SyntheticEvent, newValue: TabKey) => {
    navigate(`/networking/${newValue}`);
  };

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
        }}
      >
        <Typography variant="h4" sx={{ mb: 1 }}>
          Networking
        </Typography>
        <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
          Configure networks and packet forwarding for Subscriber traffic.
        </Typography>

        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          aria-label="Networking sections"
          sx={{ borderBottom: 1, borderColor: "divider" }}
        >
          <Tab value="data-networks" label="Data Networks" />
          <Tab value="slices" label="Slices" />
          <Tab value="interfaces" label="Interfaces" />
          <Tab value="routes" label="Routes" />
          <Tab value="nat" label="NAT" />
          <Tab value="bgp" label="BGP" />
          <Tab value="flow-accounting" label="Flow Accounting" />
        </Tabs>

        <Outlet context={{ accessToken, canEdit, showSnackbar, gridTheme }} />
      </Box>
    </Box>
  );
}
