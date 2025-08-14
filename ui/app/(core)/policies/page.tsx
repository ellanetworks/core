"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  Table,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  TableContainer,
  Paper,
  IconButton,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import { listPolicies, deletePolicy } from "@/queries/policies";
import CreatePolicyModal from "@/components/CreatePolicyModal";
import EditPolicyModal from "@/components/EditPolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { Policy } from "@/types/types";

const MAX_WIDTH = 1400;

const PolicyPage = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<Policy | null>(null);
  const [selectedPolicy, setSelectedPolicy] = useState<string | null>(null);
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";

  const fetchPolicies = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listPolicies(cookies.user_token);
      setPolicies(data);
    } catch (error) {
      console.error("Error fetching policies:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchPolicies();
  }, [fetchPolicies]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleEditClick = (policy: Policy) => {
    setEditData(policy);
    setEditModalOpen(true);
  };
  const handleDeleteClick = (policyName: string) => {
    setSelectedPolicy(policyName);
    setConfirmationOpen(true);
  };
  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedPolicy) return;
    try {
      await deletePolicy(cookies.user_token, selectedPolicy);
      setAlert({
        message: `Policy "${selectedPolicy}" deleted successfully!`,
        severity: "success",
      });
      fetchPolicies();
    } catch (error) {
      setAlert({
        message: `Failed to delete policy "${selectedPolicy}": ${
          error instanceof Error ? error.message : "Unknown error"
        }`,
        severity: "error",
      });
    } finally {
      setSelectedPolicy(null);
    }
  };

  const descriptionText =
    "Define  bitrate and priority levels for your subscribers.";

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity={alert.severity || "success"}
            onClose={() => setAlert({ message: "", severity: null })}
            sx={{ mb: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>

      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : policies.length === 0 ? (
        <EmptyState
          primaryText="No policy found."
          secondaryText="Create a new policy to control QoS and routing for subscribers."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <>
          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              px: { xs: 2, sm: 4 },
              mb: 3,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <Typography variant="h4">Policies</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>

            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={handleOpenCreateModal}
                sx={{ maxWidth: 200 }}
              >
                Create
              </Button>
            )}
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <TableContainer
              component={Paper}
              elevation={0}
              sx={{ border: 1, borderColor: "divider" }}
            >
              <Table aria-label="policies table" stickyHeader>
                <TableHead>
                  <TableRow
                    sx={{
                      "& th": {
                        fontWeight: "bold",
                        backgroundColor:
                          theme.palette.mode === "light"
                            ? "#F5F5F5"
                            : "inherit",
                      },
                    }}
                  >
                    <TableCell>Name</TableCell>
                    <TableCell>Bitrate (Up)</TableCell>
                    <TableCell>Bitrate (Down)</TableCell>
                    <TableCell sx={{ width: 80 }}>5QI</TableCell>
                    <TableCell sx={{ width: 100 }}>Priority</TableCell>
                    <TableCell>Data Network</TableCell>
                    {canEdit && <TableCell align="right">Actions</TableCell>}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {policies.map((p) => (
                    <TableRow key={p.name} hover>
                      <TableCell>{p.name}</TableCell>
                      <TableCell>{p.bitrateUp}</TableCell>
                      <TableCell>{p.bitrateDown}</TableCell>
                      <TableCell>{p.fiveQi}</TableCell>
                      <TableCell>{p.priorityLevel}</TableCell>
                      <TableCell>{p.dataNetworkName}</TableCell>
                      {canEdit && (
                        <TableCell align="right">
                          <IconButton
                            aria-label="edit"
                            onClick={() => handleEditClick(p)}
                            size="small"
                          >
                            <EditIcon color="primary" />
                          </IconButton>
                          <IconButton
                            aria-label="delete"
                            onClick={() => handleDeleteClick(p.name)}
                            size="small"
                          >
                            <DeleteIcon color="primary" />
                          </IconButton>
                        </TableCell>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        </>
      )}

      <CreatePolicyModal
        open={isCreateModalOpen}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={fetchPolicies}
      />
      <EditPolicyModal
        open={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        onSuccess={fetchPolicies}
        initialData={
          editData || {
            name: "",
            bitrateUp: "100 Mbps",
            bitrateDown: "100 Mbps",
            fiveQi: 1,
            priorityLevel: 1,
            dataNetworkName: "",
          }
        }
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the policy "${selectedPolicy}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default PolicyPage;
