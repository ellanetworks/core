// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Typography } from "@mui/material";
import PageTitle from "@/components/PageTitle";

interface ListPageHeaderProps {
  title: string;
  count?: number;
  description: string;
  variant?: "page" | "section";
  filters?: React.ReactNode;
  action?: React.ReactNode;
}

const ListPageHeader: React.FC<ListPageHeaderProps> = ({
  title,
  count,
  description,
  variant = "page",
  filters,
  action,
}) => (
  <Box sx={{ mb: 3, display: "flex", flexDirection: "column", gap: 2 }}>
    <Box>
      {variant === "page" ? (
        <PageTitle title={title} count={count} />
      ) : (
        <Typography variant="h5" component="h2">
          {count === undefined ? title : `${title} (${count})`}
        </Typography>
      )}
      <Typography
        variant={variant === "page" ? "body1" : "body2"}
        color="textSecondary"
        sx={{ mt: variant === "page" ? 2 : 0.5 }}
      >
        {description}
      </Typography>
    </Box>
    {(filters || action) && (
      <Box
        data-testid="list-page-toolbar"
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          alignItems: { xs: "stretch", sm: "center" },
          justifyContent: "space-between",
          gap: 2,
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            alignItems: { xs: "stretch", sm: "center" },
            gap: 2,
          }}
        >
          {filters}
        </Box>
        {action}
      </Box>
    )}
  </Box>
);

export default ListPageHeader;
