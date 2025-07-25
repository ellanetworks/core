"use client";

import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  IconButton,
} from "@mui/material";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listProfiles, deleteProfile } from "@/queries/profiles";
import CreateProfileModal from "@/components/CreateProfileModal";
import EditProfileModal from "@/components/EditProfileModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { Profile } from "@/types/types";

const ProfilePage = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<Profile | null>(null);
  const [selectedProfile, setSelectedProfile] = useState<string | null>(null);
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });

  const fetchProfiles = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listProfiles(cookies.user_token);
      setProfiles(data);
    } catch (error) {
      console.error("Error fetching profiles:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchProfiles();
  }, [fetchProfiles]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleCloseCreateModal = () => setCreateModalOpen(false);

  const handleEditClick = (profile: Profile) => {
    setEditData(profile);
    setEditModalOpen(true);
  };

  const handleDeleteClick = (profileName: string) => {
    setSelectedProfile(profileName);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedProfile) {
      try {
        await deleteProfile(cookies.user_token, selectedProfile);
        setAlert({
          message: `Profile "${selectedProfile}" deleted successfully!`,
          severity: "success",
        });
        fetchProfiles();
      } catch (error) {
        setAlert({
          message: `Failed to delete profile "${selectedProfile}": ${error}`,
          severity: "error",
        });
      } finally {
        setSelectedProfile(null);
      }
    }
  };

  const baseColumns: GridColDef[] = [
    { field: "name", headerName: "Name", flex: 1 },
    { field: "ipPool", headerName: "IP Pool", flex: 1 },
    { field: "dns", headerName: "DNS", flex: 1 },
    { field: "bitrateUp", headerName: "Bitrate (Up)", flex: 1 },
    { field: "bitrateDown", headerName: "Bitrate (Down)", flex: 1 },
    { field: "fiveQi", headerName: "5QI", flex: 0.5 },
    { field: "priorityLevel", headerName: "Priority", flex: 0.5 },
  ];

  if (role === "Admin" || role === "Network Manager") {
    baseColumns.push({
      field: "actions",
      headerName: "Actions",
      type: "actions",
      flex: 1,
      getActions: (params) => [
        <IconButton
          key="edit"
          aria-label="edit"
          onClick={() => handleEditClick(params.row)}
        >
          <EditIcon />
        </IconButton>,
        <IconButton
          key="delete"
          aria-label="delete"
          onClick={() => handleDeleteClick(params.row.name)}
        >
          <DeleteIcon />
        </IconButton>,
      ],
    });
  }

  return (
    <Box
      sx={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        justifyContent: "flex-start",
        alignItems: "center",
        paddingTop: 6,
        textAlign: "center",
      }}
    >
      <Box sx={{ width: "60%" }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity={alert.severity || "success"}
            onClose={() => setAlert({ message: "", severity: null })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>
      {loading ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          <CircularProgress />
        </Box>
      ) : profiles.length === 0 ? (
        <EmptyState
          primaryText="No profile found."
          secondaryText="Create a new profile in order to add subscribers to the network."
          button={role === "Admin" || role === "Network Manager"}
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <>
          <Box
            sx={{
              marginBottom: 4,
              width: "60%",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography variant="h4" component="h1" gutterBottom>
              Profiles ({profiles.length})
            </Typography>
            {(role === "Admin" || role === "Network Manager") && (
              <Button
                variant="contained"
                color="success"
                onClick={handleOpenCreateModal}
              >
                Create
              </Button>
            )}
          </Box>
          <Box
            sx={{
              height: "80vh",
              width: "60%",
              "& .MuiDataGrid-root": { border: "none" },
              "& .MuiDataGrid-cell": { borderBottom: "none" },
              "& .MuiDataGrid-columnHeaders": { borderBottom: "none" },
              "& .MuiDataGrid-footerContainer": { borderTop: "none" },
            }}
          >
            <DataGrid
              rows={profiles}
              columns={baseColumns}
              disableRowSelectionOnClick
              getRowId={(row) => row.name}
            />
          </Box>
        </>
      )}
      <CreateProfileModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={fetchProfiles}
      />
      <EditProfileModal
        open={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        onSuccess={fetchProfiles}
        initialData={
          editData || {
            name: "",
            ipPool: "",
            dns: "",
            mtu: 1500,
            bitrateUp: "100 Mbps",
            bitrateDown: "100 Mbps",
            fiveQi: 1,
            priorityLevel: 1,
          }
        }
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the profile "${selectedProfile}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default ProfilePage;
