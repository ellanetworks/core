// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import { Typography } from "@mui/material";
import type { SxProps, Theme } from "@mui/material/styles";
import { Link as RouterLink } from "react-router-dom";
import { PRODUCT } from "@/utils/product";

export interface PageTitleAncestor {
  label: string;
  to?: string;
}

export interface PageTitleProps {
  title: string;
  count?: number;
  parent?: PageTitleAncestor | PageTitleAncestor[];
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
  const ancestors =
    parent === undefined ? [] : Array.isArray(parent) ? parent : [parent];
  const name =
    documentTitle ?? [...ancestors.map((a) => a.label), title].join(" / ");

  useEffect(() => {
    const appName = PRODUCT.name;
    document.title = name === appName ? appName : `${name} · ${appName}`;
  }, [name]);

  if (ancestors.length > 0) {
    return (
      <Typography
        variant={size}
        component="h1"
        sx={[
          { display: "flex", alignItems: "baseline", gap: 0, flexWrap: "wrap" },
          ...(Array.isArray(sx) ? sx : [sx]),
        ]}
      >
        {ancestors.map((ancestor, index) => (
          <React.Fragment key={`${ancestor.label}-${index}`}>
            {ancestor.to === undefined ? (
              <Typography
                component="span"
                variant={size}
                sx={{ color: "text.secondary" }}
              >
                {ancestor.label}
              </Typography>
            ) : (
              <Typography
                component={RouterLink}
                to={ancestor.to}
                variant={size}
                sx={{
                  color: "text.secondary",
                  textDecoration: "none",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {ancestor.label}
              </Typography>
            )}
            <Typography
              component="span"
              variant={size}
              sx={{ color: "text.secondary", mx: 1 }}
            >
              /
            </Typography>
          </React.Fragment>
        ))}
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
