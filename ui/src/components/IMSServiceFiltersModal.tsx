// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { describeTriggerPoint, type IMSService } from "@/queries/ims";

interface IMSServiceFiltersModalProps {
  open: boolean;
  onClose: () => void;
  service: IMSService;
}

const handlingLabels: Record<string, string> = {
  SESSION_CONTINUED: "Continue",
  SESSION_TERMINATED: "Terminate",
};

const IMSServiceFiltersModal: React.FC<IMSServiceFiltersModalProps> = ({
  open,
  onClose,
  service,
}) => {
  const filters = service.initial_filter_criteria ?? [];
  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="md"
      fullWidth
      aria-labelledby="ims-service-filters-title"
    >
      <DialogTitle id="ims-service-filters-title">
        Filter Criteria: {service.name}
      </DialogTitle>
      <DialogContent dividers>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
          Requests matching a trigger go to {service.application_server_name}.
          If the server does not answer, the default handling applies.
        </Typography>
        {filters.length === 0 ? (
          <Typography variant="body2">No filter criteria.</Typography>
        ) : (
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 600, width: 80 }}>
                  Priority
                </TableCell>
                <TableCell sx={{ fontWeight: 600 }}>Trigger</TableCell>
                <TableCell sx={{ fontWeight: 600, width: 160 }}>
                  Default Handling
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filters.map((ifc) => (
                <TableRow key={ifc.priority}>
                  <TableCell>{ifc.priority}</TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {describeTriggerPoint(ifc)}
                  </TableCell>
                  <TableCell>
                    {handlingLabels[ifc.default_handling] ??
                      ifc.default_handling}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default IMSServiceFiltersModal;
