import React from "react";
import {
  Box,
  Card,
  CardContent,
  Chip,
  IconButton,
  Typography,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import { roleIDToLabel, type APIUser, RoleID } from "@/queries/users";

interface UserAccountCardProps {
  user: APIUser;
  canEdit?: boolean;
  onEdit?: () => void;
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
}) => {
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
      </CardContent>
    </Card>
  );
};

export default UserAccountCard;
