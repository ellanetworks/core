"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  Chip,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  GridColDef,
  GridActionsCellItem,
  GridRenderCellParams,
  GridRowParams,
} from "@mui/x-data-grid";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import VisibilityIcon from "@mui/icons-material/Visibility";
import { listSubscribers, deleteSubscriber } from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import ViewSubscriberModal from "@/components/ViewSubscriberModal";
import EditSubscriberModal from "@/components/EditSubscriberModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { Subscriber } from "@/types/types";
import { ThemeProvider, createTheme } from "@mui/material/styles";

const SubscriberPage = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [subscribers, setSubscribers] = useState<Subscriber[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isViewModalOpen, setViewModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<Subscriber | null>(null);
  const [selectedSubscriber, setSelectedSubscriber] = useState<string | null>(
    null,
  );
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const outerTheme = useTheme();

  const gridTheme = React.useMemo(
    () =>
      createTheme(outerTheme, {
        palette: {
          DataGrid: {
            headerBg: "#F5F5F5",
          },
        },
      }),
    [outerTheme],
  );

  const fetchSubscribers = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listSubscribers(cookies.user_token);
      setSubscribers(data);
    } catch (error) {
      console.error("Error fetching subscribers:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchSubscribers();
  }, [fetchSubscribers]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleCloseCreateModal = () => setCreateModalOpen(false);
  const handleCloseViewModal = () => {
    setSelectedSubscriber(null);
    setViewModalOpen(false);
  };

  const handleEditClick = (subscriber: Subscriber) => {
    setEditData(subscriber);
    setEditModalOpen(true);
  };

  const handleViewClick = (subscriber: Subscriber) => {
    setSelectedSubscriber(subscriber.imsi);
    setViewModalOpen(true);
  };

  const handleDeleteClick = (imsi: string) => {
    setSelectedSubscriber(imsi);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedSubscriber) return;
    try {
      await deleteSubscriber(cookies.user_token, selectedSubscriber);
      setAlert({
        message: `Subscriber "${selectedSubscriber}" deleted successfully!`,
        severity: "success",
      });
      fetchSubscribers();
    } catch {
      setAlert({
        message: `Failed to delete subscriber "${selectedSubscriber}".`,
        severity: "error",
      });
    } finally {
      setSelectedSubscriber(null);
    }
  };

  const columns: GridColDef<Subscriber>[] = useMemo(() => {
    const actions = (row: Subscriber) =>
      isSmDown
        ? [
            <GridActionsCellItem
              key="view"
              icon={<VisibilityIcon />}
              label="View"
              onClick={() => handleViewClick(row)}
            />,
            <GridActionsCellItem
              key="edit"
              icon={<EditIcon />}
              label="Edit"
              onClick={() => handleEditClick(row)}
              showInMenu
            />,
            <GridActionsCellItem
              key="delete"
              icon={<DeleteIcon />}
              label="Delete"
              onClick={() => handleDeleteClick(row.imsi)}
              showInMenu
            />,
          ]
        : [
            <GridActionsCellItem
              key="view"
              icon={<VisibilityIcon color={"primary"} />}
              label="View"
              onClick={() => handleViewClick(row)}
            />,
            <GridActionsCellItem
              key="edit"
              icon={<EditIcon color={"primary"} />}
              label="Edit"
              onClick={() => handleEditClick(row)}
            />,
            <GridActionsCellItem
              key="delete"
              icon={<DeleteIcon color={"primary"} />}
              label="Delete"
              onClick={() => handleDeleteClick(row.imsi)}
            />,
          ];
    const base: GridColDef<Subscriber>[] = [
      { field: "imsi", headerName: "IMSI", flex: 1, minWidth: 200 },
      { field: "policyName", headerName: "Policy", flex: 0.8, minWidth: 140 },

      {
        field: "registration",
        headerName: "Registration",
        width: 140,
        minWidth: 120,
        valueGetter: (_value, row: Subscriber) =>
          Boolean(row?.status?.registered),
        sortComparator: (v1, v2) => Number(v1) - Number(v2),
        renderCell: (params: GridRenderCellParams<Subscriber>) => {
          const registered = Boolean(params.row?.status?.registered);
          return (
            <Chip
              size="small"
              label={registered ? "Registered" : "Deregistered"}
              color={registered ? "success" : "default"}
              variant={"outlined"}
            />
          );
        },
      },
      {
        field: "session",
        headerName: "Session",
        width: 140,
        minWidth: 120,
        valueGetter: (_value, row: Subscriber) =>
          (row?.status?.sessions?.length ?? 0) > 0,
        sortComparator: (v1, v2) => Number(v1) - Number(v2),
        renderCell: (params: GridRenderCellParams<Subscriber>) => {
          const active = (params.row?.status?.sessions?.length ?? 0) > 0;
          return (
            <Chip
              size="small"
              label={active ? "Active" : "Inactive"}
              color={active ? "success" : "default"}
              variant={"outlined"}
            />
          );
        },
      },

      {
        field: "ipAddress",
        headerName: "IP Address",
        width: 140,
        minWidth: 120,
        valueGetter: (_value, row: Subscriber) =>
          row?.status?.sessions && row.status.sessions.length > 0
            ? row.status.sessions[0]?.ipAddress || ""
            : "",
        renderCell: (params: GridRenderCellParams<Subscriber>) => {
          const ip =
            params.row?.status?.sessions &&
            params.row.status.sessions.length > 0
              ? params.row.status.sessions[0]?.ipAddress || ""
              : "";

          return (
            <Chip
              size="small"
              label={ip || "N/A"}
              color={ip ? "success" : "default"}
              variant="outlined"
              sx={{ fontSize: "0.75rem" }}
            />
          );
        },
      },
    ];

    if (role === "Admin" || role === "Network Manager") {
      base.push({
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: 120,
        sortable: false,
        disableColumnMenu: true,
        getActions: (params: GridRowParams<Subscriber>) => actions(params.row),
      });
    }

    return base;
  }, [role, isSmDown]);

  const columnGroupingModel = [
    {
      groupId: "statusGroup",
      headerName: "Status",
      children: [
        { field: "registration" },
        { field: "session" },
        { field: "ipAddress" },
      ],
    },
  ];

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
      <Box sx={{ width: "100%", maxWidth: 1400, px: { xs: 2, sm: 4 } }}>
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
      ) : subscribers.length === 0 ? (
        <EmptyState
          primaryText="No subscriber found."
          secondaryText="Create a new subscriber."
          button={role === "Admin" || role === "Network Manager"}
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <>
          <Box
            sx={{
              width: "100%",
              maxWidth: 1400,
              px: { xs: 2, sm: 4 },
              mb: 3,
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
              justifyContent: "space-between",
              alignItems: { xs: "stretch", sm: "center" },
              gap: 2,
            }}
          >
            <Typography variant="h4">
              Subscribers ({subscribers.length})
            </Typography>
            {(role === "Admin" || role === "Network Manager") && (
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
            )}
          </Box>

          <Box sx={{ width: "100%", maxWidth: 1400 }}>
            <ThemeProvider theme={gridTheme}>
              <DataGrid
                rows={subscribers}
                columns={columns}
                getRowId={(row) => row.imsi}
                disableRowSelectionOnClick
                columnVisibilityModel={{}}
                columnGroupingModel={columnGroupingModel}
                sx={{
                  width: "100%",
                  height: { xs: 460, sm: 560, md: 640 },
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaders": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
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

      <ViewSubscriberModal
        open={isViewModalOpen}
        onClose={handleCloseViewModal}
        imsi={selectedSubscriber || ""}
      />
      <CreateSubscriberModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={fetchSubscribers}
      />
      <EditSubscriberModal
        open={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        onSuccess={fetchSubscribers}
        initialData={editData || { imsi: "", policyName: "" }}
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the subscriber "${selectedSubscriber}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default SubscriberPage;
