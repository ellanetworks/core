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
import {
  getProfileIMSServiceProfiles,
  imsRegistrationSetLabels,
  imsRegistrationSets,
  type ProfileIMSServiceProfiles,
} from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditProfileIMSServiceProfilesModal from "@/components/EditProfileIMSServiceProfilesModal";

interface ProfileIMSServiceProfilesCardProps {
  profileName: string;
  canEdit: boolean;
}

const ProfileIMSServiceProfilesCard: React.FC<
  ProfileIMSServiceProfilesCardProps
> = ({ profileName, canEdit }) => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const [editOpen, setEditOpen] = useState(false);

  const query = useQuery<ProfileIMSServiceProfiles>({
    queryKey: ["profile-ims-service-profiles", profileName],
    queryFn: () => getProfileIMSServiceProfiles(accessToken!, profileName),
    enabled: authReady && !!accessToken && !editOpen,
    refetchInterval: 5000,
    retry: false,
  });

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
          <Typography variant="h6">IMS Service Profiles</Typography>
          {canEdit && (
            <IconButton
              size="small"
              color="primary"
              onClick={() => setEditOpen(true)}
              aria-label="Edit IMS service profiles"
              disabled={!query.isSuccess}
            >
              <EditIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 1.5 }}>
          The service profile, and so the filter criteria, of the MMTel and
          MCPTT identities of subscribers on this profile.
        </Typography>
        {query.isLoading ? (
          <Skeleton variant="text" width={200} />
        ) : query.isError ? (
          <Typography variant="body2" color="error">
            Failed to load IMS service profiles.
          </Typography>
        ) : (
          <Table
            size="small"
            aria-label="IMS service profiles"
            sx={{ "& tr:last-child td": { borderBottom: "none" } }}
          >
            <TableBody>
              {imsRegistrationSets.map((set) => (
                <TableRow key={set}>
                  <TableCell sx={{ fontWeight: 600, width: "35%" }}>
                    {imsRegistrationSetLabels[set]}
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {query.data?.[set] ?? "None"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
      {editOpen && query.data && (
        <EditProfileIMSServiceProfilesModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            void query.refetch();
            showSnackbar(
              "IMS service profiles updated successfully.",
              "success",
            );
          }}
          profileName={profileName}
          initialData={query.data}
        />
      )}
    </Card>
  );
};

export default ProfileIMSServiceProfilesCard;
