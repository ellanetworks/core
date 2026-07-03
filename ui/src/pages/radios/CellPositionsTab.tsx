// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Stack,
  Typography,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
} from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import {
  listCellPositions,
  deleteCellPosition,
  type CellPosition,
} from "@/queries/cell_positions";
import CellPositionFormModal from "@/components/CellPositionFormModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";

import { useRadiosContext } from "./types";

const ratChip = (rat: string) => (
  <Chip
    size="small"
    label={rat === "nr" ? "NR (5G)" : "E-UTRA (4G)"}
    color={rat === "nr" ? "primary" : "secondary"}
    variant="outlined"
  />
);

export default function CellPositionsTab() {
  const { gridTheme, accessToken, canEdit, showSnackbar } = useRadiosContext();

  const {
    data: rows = [],
    isLoading,
    refetch,
  } = useQuery<CellPosition[]>({
    queryKey: ["cell-positions"],
    queryFn: () => listCellPositions(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 10000,
    refetchOnWindowFocus: true,
  });

  const [isCreateOpen, setCreateOpen] = useState(false);
  const [editRow, setEditRow] = useState<CellPosition | null>(null);
  const [deleteRow, setDeleteRow] = useState<CellPosition | null>(null);

  const handleConfirmDelete = async () => {
    if (!deleteRow || !accessToken) return;
    try {
      await deleteCellPosition(accessToken, deleteRow.id);
      showSnackbar("Cell position deleted successfully.", "success");
      refetch();
    } catch (error: unknown) {
      showSnackbar(`Failed to delete cell position: ${String(error)}`, "error");
    } finally {
      setDeleteRow(null);
    }
  };

  const columns: GridColDef<CellPosition>[] = useMemo(
    () => [
      {
        field: "rat",
        headerName: "RAT",
        width: 110,
        renderCell: (p) => ratChip(p.row.rat),
      },
      {
        field: "plmn",
        headerName: "PLMN",
        width: 100,
        valueGetter: (_v, row) => `${row.mcc}-${row.mnc}`,
      },
      {
        field: "cell_identity",
        headerName: "Cell Identity",
        flex: 1,
        minWidth: 140,
        renderCell: (p) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
              {p.row.cell_identity}
            </Typography>
          </Box>
        ),
      },
      {
        field: "gnb_id",
        headerName: "gNB ID",
        flex: 1,
        minWidth: 100,
        valueGetter: (_v, row) => row.gnb_id || "—",
      },
      {
        field: "latitude",
        headerName: "Latitude",
        width: 110,
      },
      {
        field: "longitude",
        headerName: "Longitude",
        width: 110,
      },
      {
        field: "altitude",
        headerName: "Altitude",
        width: 110,
        valueGetter: (_v, row) => row.altitude ?? "—",
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: canEdit ? 100 : 20,
        sortable: false,
        disableColumnMenu: true,
        getActions: (p: { row: CellPosition }) =>
          canEdit
            ? [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => setEditRow(p.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => setDeleteRow(p.row)}
                />,
              ]
            : [],
      } as GridColDef<CellPosition>,
    ],
    [canEdit],
  );

  const description =
    "Provisioned antenna coordinates for serving cells, used to anchor Cell-ID and E-CID location estimates when the RAN does not report its own position.";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      {isLoading && rows.length === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : rows.length === 0 ? (
        <EmptyState
          primaryText="No cell positions provisioned."
          secondaryText="Add a cell position to anchor Cell-ID / E-CID location estimates."
          extraContent={description}
          button={canEdit}
          buttonText="Add Cell Position"
          onCreate={() => setCreateOpen(true)}
          readOnlyHint={
            canEdit
              ? undefined
              : "Ask an Admin or Network Manager to provision cell positions."
          }
        />
      ) : (
        <>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            sx={{
              alignItems: { xs: "stretch", sm: "center" },
              justifyContent: "space-between",
              mb: 2,
            }}
          >
            <Box>
              <Typography variant="h5" sx={{ mb: 0.5 }}>
                Cell Positions ({rows.length})
              </Typography>
              <Typography variant="body2" color="textSecondary">
                {description}
              </Typography>
            </Box>
            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={() => setCreateOpen(true)}
                sx={{ maxWidth: 200 }}
              >
                Add Cell Position
              </Button>
            )}
          </Stack>

          <ThemeProvider theme={gridTheme}>
            <DataGrid<CellPosition>
              rows={rows}
              columns={columns}
              getRowId={(row) => row.id}
              disableColumnMenu
              disableRowSelectionOnClick
              pageSizeOptions={[10, 25, 50]}
              initialState={{
                pagination: { paginationModel: { pageSize: 25 } },
              }}
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
        <CellPositionFormModal
          open
          onClose={() => setCreateOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Cell position created successfully.", "success");
          }}
        />
      )}
      {editRow && (
        <CellPositionFormModal
          open
          onClose={() => setEditRow(null)}
          onSuccess={() => {
            refetch();
            showSnackbar("Cell position updated successfully.", "success");
          }}
          initial={editRow}
        />
      )}
      {deleteRow && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteRow(null)}
          onConfirm={handleConfirmDelete}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the cell position for ${deleteRow.cell_identity}? This action cannot be undone.`}
        />
      )}
    </Box>
  );
}
