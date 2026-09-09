// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import { Typography } from "@mui/material";
import type { SxProps, Theme } from "@mui/material/styles";
import { Link as RouterLink } from "react-router-dom";
import { PRODUCT } from "@/utils/product";

export interface PageTitleProps {
  title: string;
  count?: number;
  parent?: { label: string; to: string };
  size?: "h4" | "h5";
  documentTitle?: string;
  adornment?: React.ReactNode;
  gutterBottom?: boolean;
  sx?: SxProps<Theme>;
}

export default function PageTitle({
  title,
  count,
  parent,
  size = "h4",
  documentTitle,
  adornment,
  gutterBottom,
  sx,
}: PageTitleProps) {
  const name = documentTitle ?? (parent ? `${parent.label} / ${title}` : title);

  useEffect(() => {
    const appName = PRODUCT.name;
    document.title = name === appName ? appName : `${name} · ${appName}`;
  }, [name]);

  if (parent) {
    return (
      <Typography
        variant={size}
        component="h1"
        sx={[
          { display: "flex", alignItems: "baseline", gap: 0 },
          ...(Array.isArray(sx) ? sx : [sx]),
        ]}
      >
        <Typography
          component={RouterLink}
          to={parent.to}
          variant={size}
          sx={{
            color: "text.secondary",
            textDecoration: "none",
            "&:hover": { textDecoration: "underline" },
          }}
        >
          {parent.label}
        </Typography>
        <Typography
          component="span"
          variant={size}
          sx={{ color: "text.secondary", mx: 1 }}
        >
          /
        </Typography>
        <Typography component="span" variant={size}>
          {title}
        </Typography>
        {adornment}
      </Typography>
    );
  }

  return (
    <Typography
      variant={size}
      component="h1"
      gutterBottom={gutterBottom}
      sx={sx}
    >
      {count === undefined ? title : `${title} (${count})`}
      {adornment}
    </Typography>
  );
}
