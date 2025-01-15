"use client";

import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  IconButton,
} from "@mui/material";
import { DataGrid, GridColDef, GridRenderCellParams } from "@mui/x-data-grid";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listProfiles, deleteProfile } from "@/queries/profiles";
import CreateProfileModal from "@/components/CreateProfileModal";
import EditProfileModal from "@/components/EditProfileModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";

interface ProfileData {
  name: string;
  ipPool: string;
  dns: string;
  mtu: number;
  bitrateUpValue: number;
  bitrateUpUnit: string;
  bitrateDownValue: number;
  bitrateDownUnit: string;
  fiveQi: number;
  priorityLevel: number;
}

const Profile = () => {
  const [cookies] = useCookies(["user_token"]);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<ProfileData | null>(null);
  const [selectedProfile, setSelectedProfile] = useState<string | null>(null);
  const [alert, setAlert] = useState<{ message: string; severity: "success" | "error" | null }>({
    message: "",
    severity: null,
  });

  const fetchProfiles = async () => {
    setLoading(true);
    try {
      const data = await listProfiles(cookies.user_token);

      const mappedData = data.map((profile: any) => ({
        id: profile.name,
        name: profile.name,
        ipPool: profile["ue-ip-pool"] || "N/A",
        dns: profile.dns || "N/A",
        bitrateUp: profile["bitrate-uplink"] || "0 Mbps",
        bitrateDown: profile["bitrate-downlink"] || "0 Mbps",
        fiveQi: profile["var5qi"] || 0,
        priorityLevel: profile["priority-level"] || 0,
      }));

      setProfiles(mappedData);
    } catch (error) {
      console.error("Error fetching profiles:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProfiles();
  }, []);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleCloseCreateModal = () => setCreateModalOpen(false);

  const handleEditClick = (profile: any) => {
    setEditData({
      name: profile.name,
      ipPool: profile.ipPool,
      dns: profile.dns,
      mtu: 1500,
      bitrateUpValue: parseInt(profile.bitrateUp),
      bitrateUpUnit: profile.bitrateUp.includes("Gbps") ? "Gbps" : "Mbps",
      bitrateDownValue: parseInt(profile.bitrateDown),
      bitrateDownUnit: profile.bitrateDown.includes("Gbps") ? "Gbps" : "Mbps",
      fiveQi: profile.fiveQi,
      priorityLevel: profile.priorityLevel,
    });
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
          severity: "success"
        });
        fetchProfiles();
      } catch (error) {
        console.log("Error deleting profile:", error);
        setAlert({
          message: `Failed to delete profile "${selectedProfile}": ${error}`,
          severity: "error",
        });
      } finally {
        setSelectedProfile(null);
      }
    }
  };

  const columns: GridColDef[] = [
    { field: "name", headerName: "Name", flex: 1 },
    { field: "ipPool", headerName: "IP Pool", flex: 1 },
    { field: "dns", headerName: "DNS", flex: 1 },
    { field: "bitrateUp", headerName: "Bitrate (Up)", flex: 1 },
    { field: "bitrateDown", headerName: "Bitrate (Down)", flex: 1 },
    { field: "fiveQi", headerName: "5QI", flex: 0.5 },
    { field: "priorityLevel", headerName: "Priority", flex: 0.5 },
    {
      field: "actions",
      headerName: "Actions",
      type: "actions",
      flex: 1,
      getActions: (params) => [
        <IconButton
          aria-label="edit"
          onClick={() => handleEditClick(params.row)}
        >
          <EditIcon />
        </IconButton>,
        <IconButton
          aria-label="delete"
          onClick={() => handleDeleteClick(params.row.name)}
        >
          <DeleteIcon />
        </IconButton>
      ],
    },
  ];

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
        <Box sx={{ display: "flex", justifyContent: "center", alignItems: "center" }}>
          <CircularProgress />
        </Box>
      ) : profiles.length === 0 ? (
        <EmptyState
          primaryText="No profile found."
          secondaryText="Create a new profile in order to add subscribers to the network."
          button={true}
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
            <Button variant="contained" color="success" onClick={handleOpenCreateModal}>
              Create
            </Button>
          </Box>
          <Box
            sx={{
              height: "80vh",
              width: "60%",
              "& .MuiDataGrid-root": {
                border: "none",
              },
              "& .MuiDataGrid-cell": {
                borderBottom: "none",
              },
              "& .MuiDataGrid-columnHeaders": {
                borderBottom: "none",
              },
              "& .MuiDataGrid-footerContainer": {
                borderTop: "none",
              },
            }}
          >
            <DataGrid
              rows={profiles}
              columns={columns}
              disableRowSelectionOnClick
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
            bitrateUpValue: 100,
            bitrateUpUnit: "Mbps",
            bitrateDownValue: 100,
            bitrateDownUnit: "Mbps",
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

export default Profile;
