// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { type SyntheticEvent } from "react";
import { Box, Typography, Tabs, Tab } from "@mui/material";
import { useLocation, useNavigate } from "react-router-dom";
import PageTitle from "@/components/PageTitle";
import SubscribersTab from "@/pages/subscribers/SubscribersTab";
import RoamersTab from "@/pages/subscribers/RoamersTab";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const TAB_PATHS = ["/subscribers", "/subscribers/roamers"] as const;

type TabPath = (typeof TAB_PATHS)[number];

const SubscribersPage: React.FC = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const currentTab: TabPath =
    location.pathname === TAB_PATHS[1] ? TAB_PATHS[1] : TAB_PATHS[0];

  const handleTabChange = (_: SyntheticEvent, newValue: TabPath) => {
    navigate(newValue);
  };

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <PageTitle title="Subscribers" sx={{ mb: 1 }} />
      <Typography variant="body1" color="textSecondary" sx={{ mb: 2 }}>
        Manage the subscribers of your private network and see the roamers
        registered on it.
      </Typography>

      <Tabs
        value={currentTab}
        onChange={handleTabChange}
        aria-label="Subscribers sections"
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        <Tab value={TAB_PATHS[0]} label="Subscribers" />
        <Tab value={TAB_PATHS[1]} label="Roamers" />
      </Tabs>

      {currentTab === TAB_PATHS[0] ? <SubscribersTab /> : <RoamersTab />}
    </Box>
  );
};

export default SubscribersPage;
