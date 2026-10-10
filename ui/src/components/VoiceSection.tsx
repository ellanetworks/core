// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Box,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import type { OperatorIMS } from "@/queries/operator";
import EditOperatorIMSModal from "@/components/EditOperatorIMSModal";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { IMS_DATA_NETWORK } from "@/utils/voice";

interface VoiceSectionProps {
  ims?: OperatorIMS;
  canEdit: boolean;
  onModalOpenChange: (open: boolean) => void;
}

const VoiceSection: React.FC<VoiceSectionProps> = ({
  ims,
  canEdit,
  onModalOpenChange,
}) => {
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);

  useEffect(() => {
    onModalOpenChange(editOpen);
  }, [editOpen, onModalOpenChange]);

  const addresses = ims?.pcscfAddresses ?? [];

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        Voice
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let 4G and 5G subscribers make voice and video calls through Ella IMS.
        Each subscriber needs a phone number and a policy on the{" "}
        {IMS_DATA_NETWORK} data network.
      </Typography>

      <TableContainer sx={TABLE_CONTAINER_SX}>
        <Table>
          <TableBody>
            <TableRow>
              <TableCell sx={{ fontWeight: 600, width: "35%" }}>
                <Tooltip title="The IMS entry points handsets call" arrow>
                  <span>P-CSCF Addresses</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ width: "55%", wordBreak: "break-word" }}>
                {addresses.length > 0 ? addresses.join(", ") : "N/A"}
              </TableCell>
              <TableCell sx={{ width: "10%", textAlign: "right" }}>
                {canEdit && (
                  <Tooltip title="Edit P-CSCF addresses" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setEditOpen(true)}
                      aria-label="Edit P-CSCF addresses"
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>

      {editOpen && (
        <EditOperatorIMSModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["operator"] });
            showSnackbar("Voice settings updated successfully.", "success");
          }}
          initialData={{ pcscfAddresses: addresses }}
        />
      )}
    </Box>
  );
};

export default VoiceSection;
