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
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import {
  describeTrigger,
  getProfileIMSFilterCriteria,
  type IMSInitialFilterCriterion,
} from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditProfileIMSFilterCriteriaModal from "@/components/EditProfileIMSFilterCriteriaModal";

interface ProfileIMSFilterCriteriaCardProps {
  profileName: string;
  canEdit: boolean;
}

const handlingLabels: Record<string, string> = {
  SESSION_CONTINUED: "Continue",
  SESSION_TERMINATED: "Terminate",
};

const ProfileIMSFilterCriteriaCard: React.FC<
  ProfileIMSFilterCriteriaCardProps
> = ({ profileName, canEdit }) => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const [editOpen, setEditOpen] = useState(false);

  const query = useQuery<IMSInitialFilterCriterion[]>({
    queryKey: ["profile-ims-filter-criteria", profileName],
    queryFn: () => getProfileIMSFilterCriteria(accessToken!, profileName),
    enabled: authReady && !!accessToken && !editOpen,
    refetchInterval: 5000,
    retry: false,
  });
  const criteria = [...(query.data ?? [])].sort(
    (a, b) => a.priority - b.priority,
  );

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
          <Typography variant="h6">IMS Filter Criteria</Typography>
          {canEdit && (
            <IconButton
              size="small"
              color="primary"
              onClick={() => setEditOpen(true)}
              aria-label="Edit IMS filter criteria"
              disabled={!query.isSuccess}
            >
              <EditIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 1.5 }}>
          Requests from subscribers on this profile that match a trigger go to
          its application server. The first criterion is evaluated first.
        </Typography>
        {query.isLoading ? (
          <Skeleton variant="text" width={200} />
        ) : query.isError ? (
          <Typography variant="body2" color="error">
            Failed to load IMS filter criteria.
          </Typography>
        ) : criteria.length === 0 ? (
          <Typography variant="body2" color="textSecondary">
            No IMS filter criteria.
          </Typography>
        ) : (
          <Table
            size="small"
            aria-label="IMS filter criteria"
            sx={{ "& tr:last-child td": { borderBottom: "none" } }}
          >
            <TableHead>
              <TableRow>
                <TableCell sx={{ width: 48, fontWeight: 600 }}>#</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>
                  Application Server
                </TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Trigger</TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Default Handling</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {criteria.map((ifc, index) => (
                <TableRow key={ifc.priority}>
                  <TableCell sx={{ fontWeight: 600 }}>{index + 1}</TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {ifc.application_server_name}
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {describeTrigger(ifc.trigger)}
                  </TableCell>
                  <TableCell>
                    {ifc.default_handling
                      ? (handlingLabels[ifc.default_handling] ??
                        ifc.default_handling)
                      : "Not set"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
      {editOpen && (
        <EditProfileIMSFilterCriteriaModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            void query.refetch();
            showSnackbar(
              "IMS filter criteria updated successfully.",
              "success",
            );
          }}
          profileName={profileName}
          initialCriteria={criteria}
        />
      )}
    </Card>
  );
};

export default ProfileIMSFilterCriteriaCard;
