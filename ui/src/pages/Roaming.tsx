// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { SyntheticEvent } from "react";
import { Box, Typography, Tabs, Tab } from "@mui/material";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import PageTitle from "@/components/PageTitle";

const TAB_SEGMENTS = ["partners", "diameter-peers", "usage", "sepp"] as const;

type TabKey = (typeof TAB_SEGMENTS)[number];

export default function RoamingPage() {
  const { role, accessToken } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";
  const { showSnackbar } = useSnackbar();

  const location = useLocation();
  const navigate = useNavigate();

  const match = location.pathname.match(/^\/roaming\/([^/]+)/);
  const segment = match?.[1] as TabKey | undefined;
  const activeTab: TabKey =
    segment && (TAB_SEGMENTS as readonly string[]).includes(segment)
      ? segment
      : "partners";

  const handleTabChange = (_: SyntheticEvent, newValue: TabKey) => {
    navigate(`/roaming/${newValue}`);
  };

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <PageTitle title="Roaming" sx={{ mb: 1 }} />
      <Typography variant="body1" color="textSecondary" sx={{ mb: 2 }}>
        Let subscribers of partner networks attach to this network, with their
        traffic broken out locally.
      </Typography>

      <Tabs
        value={activeTab}
        onChange={handleTabChange}
        aria-label="Roaming sections"
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        <Tab value="partners" label="Partners" />
        <Tab value="diameter-peers" label="Diameter Peers" />
        <Tab value="usage" label="Usage" />
        <Tab value="sepp" label="SEPP" />
      </Tabs>

      <Outlet context={{ accessToken, canEdit, showSnackbar }} />
    </Box>
  );
}
