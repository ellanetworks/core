import React from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import type { APIAuditLog } from "@/queries/audit_logs";
import { formatDateTime } from "@/utils/formatters";

interface UserAuditLogsCardProps {
  logs: APIAuditLog[];
  email: string;
}

const UserAuditLogsCard: React.FC<UserAuditLogsCardProps> = ({
  logs,
  email,
}) => {
  return (
    <Card variant="outlined">
      <CardContent>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            mb: 2,
          }}
        >
          <Typography variant="h6">Recent Audit Logs</Typography>
          <Button
            component={RouterLink}
            to={`/audit-logs?user=${encodeURIComponent(email)}`}
            size="small"
            sx={{
              color: (theme) => theme.palette.link,
              textDecoration: "underline",
              "&:hover": { textDecoration: "underline" },
            }}
          >
            View Audit Logs for {email} &rarr;
          </Button>
        </Box>

        {logs.length === 0 ? (
          <Typography variant="body2" color="textSecondary">
            No audit log entries for this user.
          </Typography>
        ) : (
          <TableContainer component={Paper}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Timestamp</TableCell>
                  <TableCell>Action</TableCell>
                  <TableCell>IP</TableCell>
                  <TableCell>Details</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {logs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {formatDateTime(log.timestamp)}
                    </TableCell>
                    <TableCell>{log.action}</TableCell>
                    <TableCell>{log.ip}</TableCell>
                    <TableCell>{log.details}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </CardContent>
    </Card>
  );
};

export default UserAuditLogsCard;
