import { useState, useMemo } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Stack,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  listSlices,
  deleteSlice,
  type ListSlicesResponse,
  type APISlice,
} from "@/queries/slices";
import CreateSliceModal from "@/components/CreateSliceModal";
import EditSliceModal from "@/components/EditSliceModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useNetworkingContext } from "./types";

export default function SlicesTab() {
  const { accessToken, canEdit, showSnackbar, gridTheme } =
    useNetworkingContext();
  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const {
    data: slicePage,
    isLoading: loading,
    refetch,
  } = useQuery<ListSlicesResponse>({
    queryKey: ["slices", pagination.page, pagination.pageSize],
    queryFn: () =>
      listSlices(accessToken || "", pagination.page + 1, pagination.pageSize),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const rows: APISlice[] = slicePage?.items ?? [];
  const rowCount = slicePage?.total_count ?? 0;

  const [isCreateOpen, setCreateOpen] = useState(false);
  const [isEditOpen, setEditOpen] = useState(false);
  const [editSlice, setEditSlice] = useState<APISlice | null>(null);
  const [isDeleteOpen, setDeleteOpen] = useState(false);
  const [selectedName, setSelectedName] = useState<string | null>(null);

  const handleRequestEdit = (slice: APISlice) => {
    setEditSlice(slice);
    setEditOpen(true);
  };

  const handleRequestDelete = (name: string) => {
    setSelectedName(name);
    setDeleteOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (!selectedName || !accessToken) return;
    try {
      await deleteSlice(accessToken, selectedName);
      setDeleteOpen(false);
      showSnackbar(`Slice "${selectedName}" deleted successfully.`, "success");
      refetch();
    } catch (error: unknown) {
      setDeleteOpen(false);
      showSnackbar(
        `Failed to delete slice "${selectedName}": ${String(error)}`,
        "error",
      );
    } finally {
      setSelectedName(null);
    }
  };

  const description =
    "Network slices identify logical network partitions using a Slice/Service Type (SST) and an optional Slice Differentiator (SD). Ella Core uses slice information alongside the data network name to determine which policies apply to a subscriber's session.";

  const columns: GridColDef<APISlice>[] = useMemo(() => {
    return [
      { field: "name", headerName: "Name", flex: 1, minWidth: 200 },
      { field: "sst", headerName: "SST", width: 100 },
      { field: "sd", headerName: "SD", flex: 1, minWidth: 140 },
      ...(canEdit
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 120,
              sortable: false,
              disableColumnMenu: true,
              getActions: (p: { row: APISlice }) => [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => handleRequestEdit(p.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleRequestDelete(p.row.name)}
                />,
              ],
            } as GridColDef<APISlice>,
          ]
        : []),
    ];
  }, [canEdit]);

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      {loading && rowCount === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : rowCount === 0 ? (
        <EmptyState
          primaryText="No network slice found."
          secondaryText="Create a network slice to get started. Ella Core uses slice information alongside the data network name to determine which policies apply to a subscriber's session."
          extraContent={
            <Typography variant="body1" color="textSecondary">
              {description}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={() => setCreateOpen(true)}
          readOnlyHint="Ask an administrator to create a network slice."
        />
      ) : (
        <>
          <Box sx={{ mb: 3 }}>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              sx={{
                alignItems: { xs: "stretch", sm: "center" },
                justifyContent: "space-between",
              }}
            >
              <Box>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Network Slices ({rowCount})
                </Typography>
                <Typography variant="body2" color="textSecondary">
                  {description}
                </Typography>
                {canEdit && (
                  <Button
                    variant="contained"
                    color="success"
                    onClick={() => setCreateOpen(true)}
                    sx={{ maxWidth: 200, mt: 2 }}
                  >
                    Create
                  </Button>
                )}
              </Box>
            </Stack>
          </Box>
          <ThemeProvider theme={gridTheme}>
            <DataGrid<APISlice>
              rows={rows}
              columns={columns}
              getRowId={(row) => row.name}
              paginationMode="server"
              rowCount={rowCount}
              paginationModel={pagination}
              onPaginationModelChange={setPagination}
              pageSizeOptions={[10, 25, 50, 100]}
              disableColumnMenu
              disableRowSelectionOnClick
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
              }}
            />
          </ThemeProvider>
        </>
      )}

      {isCreateOpen && (
        <CreateSliceModal
          open
          onClose={() => setCreateOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Network slice created successfully.", "success");
          }}
        />
      )}
      {isEditOpen && editSlice && (
        <EditSliceModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Network slice updated successfully.", "success");
          }}
          initialData={editSlice}
        />
      )}
      {isDeleteOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteOpen(false)}
          onConfirm={handleConfirmDelete}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the slice "${selectedName}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
}
