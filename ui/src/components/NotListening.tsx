// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Tooltip, Typography } from "@mui/material";

export const notListeningReason = (
  setting: string,
  configured: boolean,
  error?: string,
) =>
  error
    ? error
    : configured
      ? "The listener has not started."
      : `Set ${setting} in the configuration file.`;

const NotListening: React.FC<{
  setting: string;
  configured?: boolean;
  error?: string;
}> = ({ setting, configured = false, error }) => (
  <Tooltip title={notListeningReason(setting, configured, error)} arrow>
    <Typography variant="body2" color="textSecondary" component="span">
      Not listening
    </Typography>
  </Tooltip>
);

export default NotListening;
