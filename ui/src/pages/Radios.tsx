// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { SyntheticEvent } from "react";
import { Box, Tabs, Tab } from "@mui/material";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X as PAGE_PAD } from "@/utils/layout";

const TAB_SEGMENTS = ["list", "events"] as const;
type TabKey = (typeof TAB_SEGMENTS)[number];

export default function RadiosPage() {
  const location = useLocation();
  const navigate = useNavigate();

  const match = location.pathname.match(/^\/radios\/([^/]+)/);
  const segment = match?.[1] as TabKey | undefined;
  const activeTab: TabKey =
    segment && (TAB_SEGMENTS as readonly string[]).includes(segment)
      ? segment
      : "list";

  const handleTabChange = (_: SyntheticEvent, newValue: TabKey) => {
    navigate(newValue === "list" ? "/radios" : `/radios/${newValue}`);
  };

  return (
    <Box sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PAD }}>
      <Tabs
        value={activeTab}
        onChange={handleTabChange}
        aria-label="Radios tabs"
        sx={{ borderBottom: 1, borderColor: "divider", mt: 2 }}
      >
        <Tab value="list" label="Radios" />
        <Tab value="events" label="Events" />
      </Tabs>

      <Outlet />
    </Box>
  );
}
