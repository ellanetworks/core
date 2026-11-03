// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Alert,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@/queries/utils";
import { formatPLMN, getRoamer, type Roamer } from "@/queries/roaming";
import { registrationState } from "@/utils/roamers";
import { formatBytesAutoUnit, formatDateTime } from "@/utils/formatters";

interface ViewRoamerModalProps {
  open: boolean;
  onClose: () => void;
  roamer: Roamer;
  accessToken: string;
}

const Row = ({ label, value }: { label: string; value: React.ReactNode }) => (
  <Stack direction="row" spacing={2} sx={{ justifyContent: "space-between" }}>
    <Typography variant="body2" color="textSecondary" sx={{ flexShrink: 0 }}>
      {label}
    </Typography>
    <Typography
      variant="body2"
      component="div"
      sx={{ textAlign: "right", wordBreak: "break-all" }}
    >
      {value}
    </Typography>
  </Stack>
);

const dash = (value?: string) => value || "—";

const ViewRoamerModal: React.FC<ViewRoamerModalProps> = ({
  open,
  onClose,
  roamer: initial,
  accessToken,
}) => {
  const roamerQuery = useQuery<Roamer>({
    queryKey: ["roaming-roamer", initial.imsi],
    queryFn: () => getRoamer(accessToken, initial.imsi),
    enabled: open && !!accessToken,
    initialData: initial,
    initialDataUpdatedAt: 0,
    refetchInterval: (query) =>
      query.state.error instanceof ApiError && query.state.error.status === 404
        ? false
        : 5000,
    retry: false,
  });

  const gone =
    roamerQuery.error instanceof ApiError && roamerQuery.error.status === 404;
  const roamer = roamerQuery.data ?? initial;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="view-roamer-title"
      maxWidth="md"
      fullWidth
    >
      <DialogTitle id="view-roamer-title">Roamer — {roamer.imsi}</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
          {gone && (
            <Alert severity="info">This roamer is no longer registered.</Alert>
          )}
          <Row label="IMSI" value={roamer.imsi} />
          <Row label="Partner" value={roamer.partnerName} />
          <Row label="Home PLMN" value={formatPLMN(roamer.homePLMN)} />
          {roamer.usageToday && (
            <Row
              label="Usage Today (UTC)"
              value={`${formatBytesAutoUnit(roamer.usageToday.totalBytes)} (${formatBytesAutoUnit(roamer.usageToday.downlinkBytes)} down, ${formatBytesAutoUnit(roamer.usageToday.uplinkBytes)} up)`}
            />
          )}

          <Divider />
          <Typography variant="subtitle2">Registrations</Typography>
          {roamer.registrations.length ? (
            <TableContainer>
              <Table size="small" aria-label="Registrations">
                <TableHead>
                  <TableRow>
                    <TableCell>RAT</TableCell>
                    <TableCell>Node</TableCell>
                    <TableCell>State</TableCell>
                    <TableCell>Radio</TableCell>
                    <TableCell>Since</TableCell>
                    <TableCell>Last Seen</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {roamer.registrations.map((reg) => {
                    const state = registrationState(reg);
                    return (
                      <TableRow key={`${reg.rat}-${reg.nodeID}`}>
                        <TableCell>{reg.rat}</TableCell>
                        <TableCell>{dash(reg.nodeID)}</TableCell>
                        <TableCell>
                          <Chip
                            size="small"
                            label={state.label}
                            color={state.color}
                          />
                        </TableCell>
                        <TableCell>{dash(reg.radio)}</TableCell>
                        <TableCell>
                          {reg.since ? formatDateTime(reg.since) : "—"}
                        </TableCell>
                        <TableCell>
                          {reg.lastSeenAt
                            ? formatDateTime(reg.lastSeenAt)
                            : "—"}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          ) : (
            <Typography variant="body2" color="textSecondary">
              No registration.
            </Typography>
          )}

          <Divider />
          <Typography variant="subtitle2">Sessions</Typography>
          {roamer.sessions.length ? (
            <TableContainer>
              <Table size="small" aria-label="Sessions">
                <TableHead>
                  <TableRow>
                    <TableCell>RAT</TableCell>
                    <TableCell>Node</TableCell>
                    <TableCell>ID</TableCell>
                    <TableCell>Data Network</TableCell>
                    <TableCell>IPv4 Address</TableCell>
                    <TableCell>IPv6 Prefix</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {roamer.sessions.map((session) => (
                    <TableRow
                      key={`${session.rat}-${session.nodeID}-${session.id}`}
                    >
                      <TableCell>{session.rat}</TableCell>
                      <TableCell>{dash(session.nodeID)}</TableCell>
                      <TableCell>{session.id}</TableCell>
                      <TableCell>{session.dataNetwork}</TableCell>
                      <TableCell>{dash(session.ipv4Address)}</TableCell>
                      <TableCell>{dash(session.ipv6Prefix)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          ) : (
            <Typography variant="body2" color="textSecondary">
              No active session.
            </Typography>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default ViewRoamerModal;
