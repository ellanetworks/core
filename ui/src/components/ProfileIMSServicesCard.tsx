// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Box,
  Card,
  CardContent,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Typography,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import { getProfileIMSServices } from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditProfileIMSServicesModal from "@/components/EditProfileIMSServicesModal";

interface ProfileIMSServicesCardProps {
  profileName: string;
  canEdit: boolean;
}

const ProfileIMSServicesCard: React.FC<ProfileIMSServicesCardProps> = ({
  profileName,
  canEdit,
}) => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const [editOpen, setEditOpen] = useState(false);

  const query = useQuery<string[]>({
    queryKey: ["profile-ims-services", profileName],
    queryFn: () => getProfileIMSServices(accessToken!, profileName),
    enabled: authReady && !!accessToken && !editOpen,
    refetchInterval: 5000,
    retry: false,
  });
  const services = query.data ?? [];

  return (
    <Card variant="outlined" sx={{ mt: 3 }}>
      <CardContent>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            mb: 0.5,
          }}
        >
          <Typography variant="h6">IMS Services</Typography>
          {canEdit && (
            <IconButton
              size="small"
              color="primary"
              onClick={() => setEditOpen(true)}
              aria-label="Edit IMS services"
              disabled={!query.isSuccess}
            >
              <EditIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 1.5 }}>
          IMS services for subscribers on this profile. Order decides iFC
          priority: the first service is evaluated first.
        </Typography>
        {query.isLoading ? (
          <Skeleton variant="text" width={200} />
        ) : query.isError ? (
          <Typography variant="body2" color="error">
            Failed to load IMS services.
          </Typography>
        ) : services.length === 0 ? (
          <Typography variant="body2" color="textSecondary">
            No IMS services.
          </Typography>
        ) : (
          <Table
            size="small"
            aria-label="IMS services"
            sx={{ "& tr:last-child td": { borderBottom: "none" } }}
          >
            <TableBody>
              {services.map((name, index) => (
                <TableRow key={name}>
                  <TableCell sx={{ width: 48, fontWeight: 600 }}>
                    {index + 1}
                  </TableCell>
                  <TableCell>{name}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
      {editOpen && (
        <EditProfileIMSServicesModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            void query.refetch();
            showSnackbar("IMS services updated successfully.", "success");
          }}
          profileName={profileName}
          initialServices={services}
        />
      )}
    </Card>
  );
};

export default ProfileIMSServicesCard;
