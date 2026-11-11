// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { SyntheticEvent } from "react";
import { Box, Typography, Tabs, Tab } from "@mui/material";
import { useAuth } from "@/contexts/AuthContext";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import PageTitle from "@/components/PageTitle";
import type { IntegrationsTabProps } from "@/pages/integrations/types";

const TAB_SEGMENTS = ["ims", "sms", "roaming"] as const;

type TabKey = (typeof TAB_SEGMENTS)[number];

export default function IntegrationsPage() {
  const { role } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const location = useLocation();
  const navigate = useNavigate();

  const match = location.pathname.match(/^\/integrations\/([^/]+)/);
  const segment = match?.[1] as TabKey | undefined;
  const activeTab: TabKey =
    segment && (TAB_SEGMENTS as readonly string[]).includes(segment)
      ? segment
      : "ims";

  const handleTabChange = (_: SyntheticEvent, newValue: TabKey) => {
    navigate(`/integrations/${newValue}`);
  };

  const context: IntegrationsTabProps = { canEdit };

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <PageTitle title="Integrations" sx={{ mb: 1 }} />
      <Typography variant="body1" color="textSecondary" sx={{ mb: 2 }}>
        Connect your network to an IMS, SMS service centres and roaming
        partners.
      </Typography>

      <Tabs
        value={activeTab}
        onChange={handleTabChange}
        aria-label="Integrations sections"
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        <Tab value="ims" label="IMS" />
        <Tab value="sms" label="SMS" />
        <Tab value="roaming" label="Roaming" />
      </Tabs>

      <Outlet context={context} />
    </Box>
  );
}
