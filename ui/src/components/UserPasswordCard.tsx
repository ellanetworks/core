// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Button, Card, CardContent, Stack, Typography } from "@mui/material";

interface UserPasswordCardProps {
  onChangePassword?: () => void;
  onRemovePassword?: () => void;
  disabled?: boolean;
  description?: string;
  changeLabel?: string;
}

const UserPasswordCard: React.FC<UserPasswordCardProps> = ({
  onChangePassword,
  onRemovePassword,
  disabled,
  description = "Keep this account secure by using a strong password and updating it periodically.",
  changeLabel = "Change Password",
}) => {
  return (
    <Card variant="outlined" sx={{ height: "100%" }}>
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Password
        </Typography>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
          {description}
        </Typography>
        <Stack direction="row" spacing={1}>
          {onChangePassword && (
            <Button
              variant="contained"
              color="success"
              onClick={onChangePassword}
              disabled={disabled}
            >
              {changeLabel}
            </Button>
          )}
          {onRemovePassword && (
            <Button
              variant="outlined"
              color="error"
              onClick={onRemovePassword}
              disabled={disabled}
            >
              Remove Password
            </Button>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
};

export default UserPasswordCard;
