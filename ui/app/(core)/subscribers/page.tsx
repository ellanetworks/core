"use client";

import React, { useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  Chip,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  GridColDef,
  GridActionsCellItem,
  GridRenderCellParams,
  GridRowParams,
  GridPaginationModel,
} from "@mui/x-data-grid";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import VisibilityIcon from "@mui/icons-material/Visibility";
import {
  listSubscribers,
  deleteSubscriber,
  type APISubscriber,
  type ListSubscribersResponse,
} from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import ViewSubscriberModal from "@/components/ViewSubscriberModal";
import EditSubscriberModal from "@/components/EditSubscriberModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";

const MAX_WIDTH = 1400;

const SubscriberPage: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));
  const canEdit = role === "Admin" || role === "Network Manager";

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isViewModalOpen, setViewModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<APISubscriber | null>(null);
  const [selectedSubscriber, setSelectedSubscriber] = useState<string | null>(
    null,
  );
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["subscribers", accessToken, pageOneBased, perPage],
    queryFn: (): Promise<ListSubscribersResponse> =>
      listSubscribers(accessToken || "", pageOneBased, perPage),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const rows: APISubscriber[] = data?.items ?? [];
  const rowCount = data?.total_count ?? 0;

  const handleCloseViewModal = () => {
    setSelectedSubscriber(null);
    setViewModalOpen(false);
  };

  const handleEditClick = (subscriber: APISubscriber) => {
    setEditData(subscriber);
    setEditModalOpen(true);
  };

  const handleViewClick = (subscriber: APISubscriber) => {
    setSelectedSubscriber(subscriber.imsi);
    setViewModalOpen(true);
  };

  const handleDeleteClick = (imsi: string) => {
    setSelectedSubscriber(imsi);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedSubscriber || !accessToken) return;
    try {
      await deleteSubscriber(accessToken, selectedSubscriber);
      setAlert({
        message: `Subscriber "${selectedSubscriber}" deleted successfully!`,
        severity: "success",
      });
      refetch();
    } catch {
      setAlert({
        message: `Failed to delete subscriber "${selectedSubscriber}".`,
        severity: "error",
      });
    } finally {
      setSelectedSubscriber(null);
    }
  };

  const columns: GridColDef<APISubscriber>[] = useMemo(() => {
    const actions = (row: APISubscriber) =>
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
              icon={<VisibilityIcon color="primary" />}
              label="View"
              onClick={() => handleViewClick(row)}
            />,
            <GridActionsCellItem
              key="edit"
              icon={<EditIcon color="primary" />}
              label="Edit"
              onClick={() => handleEditClick(row)}
            />,
            <GridActionsCellItem
              key="delete"
              icon={<DeleteIcon color="primary" />}
              label="Delete"
              onClick={() => handleDeleteClick(row.imsi)}
            />,
          ];

    const base: GridColDef<APISubscriber>[] = [
      { field: "imsi", headerName: "IMSI", flex: 1, minWidth: 200 },
      { field: "policyName", headerName: "Policy", flex: 0.8, minWidth: 140 },
      {
        field: "registration",
        headerName: "Registration",
        width: 140,
        minWidth: 120,
        valueGetter: (_v, row) => Boolean(row?.status?.registered),
        sortComparator: (v1, v2) => Number(v1) - Number(v2),
        renderCell: (params: GridRenderCellParams<APISubscriber>) => {
          const registered = Boolean(params.row?.status?.registered);
          return (
            <Chip
              size="small"
              label={registered ? "Registered" : "Deregistered"}
              color={registered ? "success" : "default"}
              variant="filled"
            />
          );
        },
      },
      {
        field: "session",
        headerName: "Session",
        width: 140,
        minWidth: 120,
        valueGetter: (_v, row: APISubscriber) =>
          Boolean(row?.status?.ipAddress),
        sortComparator: (v1, v2) => Number(Boolean(v1)) - Number(Boolean(v2)),
        renderCell: (params: GridRenderCellParams<APISubscriber>) => {
          const active = Boolean(params.row?.status?.ipAddress);
          return (
            <Chip
              size="small"
              label={active ? "Active" : "Inactive"}
              color={active ? "success" : "default"}
              variant="filled"
            />
          );
        },
      },
      {
        field: "ipAddress",
        headerName: "IP Address",
        width: 140,
        minWidth: 120,
        valueGetter: (_v, row: APISubscriber) => row?.status?.ipAddress ?? "",
        renderCell: (params: GridRenderCellParams<APISubscriber>) => {
          const ip = params.row?.status?.ipAddress ?? "";
          return (
            <Chip
              size="small"
              label={ip || "N/A"}
              color={ip ? "success" : "default"}
              variant="filled"
              sx={{ fontSize: "0.75rem" }}
            />
          );
        },
      },
    ];

    if (canEdit) {
      base.push({
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: 120,
        sortable: false,
        disableColumnMenu: true,
        getActions: (params: GridRowParams<APISubscriber>) =>
          actions(params.row),
      });
    }

    return base;
  }, [canEdit, isSmDown]);

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

  const descriptionText =
    "Manage subscribers connecting to your private network. After creating a subscriber here, you can emit a SIM card with the corresponding IMSI, Key and OPc.";

  if (!authReady) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

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

      {!isLoading && rowCount === 0 ? (
        <EmptyState
          primaryText="No subscriber found."
          secondaryText="Create a new subscriber."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={() => setCreateModalOpen(true)}
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
            <Typography variant="h4">Subscribers ({rowCount})</Typography>
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={() => setCreateModalOpen(true)}
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
              <DataGrid<APISubscriber>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.imsi}
                columnGroupingModel={columnGroupingModel}
                disableRowSelectionOnClick
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={[10, 25, 50, 100]}
                sortingMode="server"
                disableColumnMenu
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

      {isViewModalOpen && (
        <ViewSubscriberModal
          open
          onClose={handleCloseViewModal}
          imsi={selectedSubscriber || ""}
        />
      )}
      {isCreateModalOpen && (
        <CreateSubscriberModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={refetch}
        />
      )}
      {isEditModalOpen && (
        <EditSubscriberModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={refetch}
          initialData={editData || { imsi: "", policyName: "" }}
        />
      )}
      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the subscriber "${selectedSubscriber}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default SubscriberPage;
