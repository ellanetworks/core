"use client";

import React, { useMemo, useState, useCallback, useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";

import {
  listPolicies,
  deletePolicy,
  type APIPolicy,
  type ListPoliciesResponse,
} from "@/queries/policies";
import CreatePolicyModal from "@/components/CreatePolicyModal";
import EditPolicyModal from "@/components/EditPolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";

const MAX_WIDTH = 1400;

const PolicyPage = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const theme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [pageData, setPageData] = useState<ListPoliciesResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<APIPolicy | null>(null);
  const [selectedPolicy, setSelectedPolicy] = useState<string | null>(null);

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const descriptionText =
    "Define bitrate and priority levels for your subscribers.";

  const fetchPolicies = useCallback(async () => {
    if (!authReady || !accessToken) return;
    setLoading(true);
    try {
      const pageOneBased = pagination.page + 1;
      const data = await listPolicies(
        accessToken,
        pageOneBased,
        pagination.pageSize,
      );
      setPageData(data);
    } catch (error) {
      console.error("Error fetching policies:", error);
      setAlert({
        message: "Failed to fetch policies.",
        severity: "error",
      });
    } finally {
      setLoading(false);
    }
  }, [accessToken, authReady, pagination.page, pagination.pageSize]);

  useEffect(() => {
    fetchPolicies();
  }, [fetchPolicies]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleEditClick = (policy: APIPolicy) => {
    setEditData(policy);
    setEditModalOpen(true);
  };
  const handleDeleteClick = (policyName: string) => {
    setSelectedPolicy(policyName);
    setConfirmationOpen(true);
  };
  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedPolicy || !authReady || !accessToken) return;
    try {
      await deletePolicy(accessToken, selectedPolicy);
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

  const rows: APIPolicy[] = pageData?.items ?? [];
  const rowCount = pageData?.total_count ?? 0;

  const columns: GridColDef<APIPolicy>[] = useMemo(() => {
    return [
      { field: "name", headerName: "Name", flex: 1, minWidth: 180 },
      {
        field: "bitrate_uplink",
        headerName: "Bitrate (Up)",
        flex: 1,
        minWidth: 160,
      },
      {
        field: "bitrate_downlink",
        headerName: "Bitrate (Down)",
        flex: 1,
        minWidth: 160,
      },
      { field: "var5qi", headerName: "5QI", width: 90 },
      { field: "arp", headerName: "ARP", width: 110 },
      {
        field: "data_network_name",
        headerName: "Data Network",
        flex: 1,
        minWidth: 160,
      },
      ...(canEdit
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 120,
              sortable: false,
              disableColumnMenu: true,
              getActions: (params) => [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => handleEditClick(params.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleDeleteClick(params.row.name)}
                />,
              ],
            } as GridColDef<APIPolicy>,
          ]
        : []),
    ];
  }, [canEdit]);

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

      {loading && rowCount === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : rowCount === 0 ? (
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
            <Typography variant="h4">Policies ({rowCount})</Typography>

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
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APIPolicy>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.name}
                loading={loading}
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={pagination}
                onPaginationModelChange={setPagination}
                pageSizeOptions={[10, 25, 50, 100]}
                disableRowSelectionOnClick
                disableColumnMenu
                autoHeight
                sx={{
                  width: "100%",
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaders": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                    backgroundColor:
                      theme.palette.mode === "light" ? "#F5F5F5" : "inherit",
                  },
                  "& .MuiDataGrid-footerContainer": {
                    borderTop: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                }}
              />
            </ThemeProvider>
          </Box>
        </>
      )}

      {isCreateModalOpen && (
        <CreatePolicyModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={fetchPolicies}
        />
      )}
      {isEditModalOpen && (
        <EditPolicyModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={fetchPolicies}
          initialData={
            editData || {
              name: "",
              bitrate_uplink: "100 Mbps",
              bitrate_downlink: "100 Mbps",
              var5qi: 1,
              arp: 1,
              data_network_name: "",
            }
          }
        />
      )}
      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the policy "${selectedPolicy}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default PolicyPage;
