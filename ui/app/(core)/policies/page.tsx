"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef, GridActionsCellItem } from "@mui/x-data-grid";
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
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

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
        message: `Failed to delete policy "${selectedPolicy}: ${
          error instanceof Error ? error.message : "Unknown error"
        }".`,
        severity: "error",
      });
    } finally {
      setSelectedPolicy(null);
    }
  };

  const columns: GridColDef[] = useMemo(() => {
    const cols: GridColDef[] = [
      { field: "name", headerName: "Name", flex: 1, minWidth: 180 },
      {
        field: "bitrateUp",
        headerName: "Bitrate (Up)",
        flex: 0.8,
        minWidth: 140,
      },
      {
        field: "bitrateDown",
        headerName: "Bitrate (Down)",
        flex: 0.8,
        minWidth: 150,
      },
      { field: "fiveQi", headerName: "5QI", flex: 0.4, minWidth: 80 },
      {
        field: "priorityLevel",
        headerName: "Priority",
        flex: 0.5,
        minWidth: 100,
      },
      {
        field: "dataNetworkName",
        headerName: "Data Network Name",
        flex: 1,
        minWidth: 180,
      },
    ];

    if (canEdit) {
      cols.push({
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: isSmDown ? 80 : 140,
        sortable: false,
        disableColumnMenu: true,
        getActions: (params) =>
          isSmDown
            ? [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon />}
                  label="Edit"
                  onClick={() => handleEditClick(params.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon />}
                  label="Delete"
                  onClick={() => handleDeleteClick(params.row.name)}
                  showInMenu
                />,
              ]
            : [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon />}
                  label="Edit"
                  onClick={() => handleEditClick(params.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon />}
                  label="Delete"
                  onClick={() => handleDeleteClick(params.row.name)}
                />,
              ],
      });
    }

    return cols;
  }, [canEdit, isSmDown]);

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      {/* Alerts */}
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
          secondaryText="Create a new policy in order to add subscribers to the network."
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
              flexDirection: { xs: "column", sm: "row" },
              justifyContent: "space-between",
              alignItems: { xs: "flex-start", sm: "center" },
              gap: 2,
            }}
          >
            <Typography variant="h4">Policies ({policies.length})</Typography>
            <Button
              variant="contained"
              color="success"
              onClick={handleOpenCreateModal}
              sx={{
                maxWidth: "200px",
                width: "100%",
              }}
            >
              Create
            </Button>
          </Box>

          <Box sx={{ width: "100%", maxWidth: MAX_WIDTH }}>
            <DataGrid
              rows={policies}
              columns={columns}
              getRowId={(row) => row.name}
              disableRowSelectionOnClick
              density="compact"
              sx={{
                height: { xs: 460, sm: 560, md: 640 },
                width: "100%",
                border: "none",
                "& .MuiDataGrid-cell": { borderBottom: "none" },
                "& .MuiDataGrid-columnHeaders": { borderBottom: "none" },
                "& .MuiDataGrid-footerContainer": { borderTop: "none" },
              }}
            />
          </Box>
        </>
      )}

      {/* Modals */}
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
