// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import {
  NOT_LINKED_YET,
  roleIDToLabel,
  signInMethods,
  type APIUser,
  RoleID,
} from "@/queries/users";

interface UserAccountCardProps {
  user: APIUser;
  canEdit?: boolean;
  onEdit?: () => void;
  fromGroups?: boolean;
  onUnlink?: () => void;
}

const InfoRow: React.FC<{ label: string; value?: React.ReactNode }> = ({
  label,
  value,
}) => {
  const isEmpty = value === undefined || value === "" || value === null;
  const display = isEmpty ? "—" : value;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        py: 0.75,
        "&:not(:last-child)": {
          borderBottom: "1px solid",
          borderColor: "divider",
        },
      }}
    >
      <Typography
        variant="body2"
        sx={{ color: "text.secondary", minWidth: 120, flexShrink: 0, mr: 2 }}
      >
        {label}
      </Typography>
      {typeof display === "string" || typeof display === "number" ? (
        <Typography variant="body2">{display}</Typography>
      ) : (
        display
      )}
    </Box>
  );
};

const roleChipColor = (roleId: RoleID): "primary" | "success" | "default" => {
  switch (roleId) {
    case RoleID.Admin:
      return "primary";
    case RoleID.NetworkManager:
      return "success";
    default:
      return "default";
  }
};

const UserAccountCard: React.FC<UserAccountCardProps> = ({
  user,
  canEdit,
  onEdit,
  fromGroups = false,
  onUnlink,
}) => {
  const methods = signInMethods(user);
  return (
    <Card variant="outlined" sx={{ height: "100%" }}>
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Account
        </Typography>
        <InfoRow label="Email" value={user.email} />
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            py: 0.75,
            borderBottom: "1px solid",
            borderColor: "divider",
          }}
        >
          <Typography
            variant="body2"
            sx={{
              color: "text.secondary",
              minWidth: 120,
              flexShrink: 0,
              mr: 2,
            }}
          >
            Role
          </Typography>
          <Chip
            size="small"
            label={roleIDToLabel(user.role_id)}
            color={roleChipColor(user.role_id)}
            variant="outlined"
          />
          {user.role_pinned && (
            <Tooltip title="Not updated from the user's groups.">
              <Chip size="small" label="Pinned" sx={{ ml: 1 }} />
            </Tooltip>
          )}
          {fromGroups && (
            <Tooltip title="Updated from the user's groups at each login.">
              <Chip size="small" label="From groups" sx={{ ml: 1 }} />
            </Tooltip>
          )}
          <Box sx={{ flex: 1 }} />
          {canEdit && onEdit && (
            <IconButton
              size="small"
              onClick={onEdit}
              aria-label="Edit role"
              color="primary"
            >
              <EditIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", py: 0.75 }}>
          <Typography
            variant="body2"
            sx={{
              color: "text.secondary",
              minWidth: 120,
              flexShrink: 0,
              mr: 2,
            }}
          >
            Sign-in
          </Typography>
          <Stack direction="row" spacing={1}>
            {methods.map((method) => (
              <Chip
                key={method}
                size="small"
                label={method}
                variant="outlined"
              />
            ))}
            {methods.length === 0 && (
              <Typography variant="body2" color="textSecondary">
                {NOT_LINKED_YET}
              </Typography>
            )}
          </Stack>
          <Box sx={{ flex: 1 }} />
          {onUnlink && user.oidc_linked && (
            <Button size="small" color="error" onClick={onUnlink}>
              Unlink
            </Button>
          )}
        </Box>
      </CardContent>
    </Card>
  );
};

export default UserAccountCard;
