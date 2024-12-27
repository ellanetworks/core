"use client";
import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  TableContainer,
  Table,
  TableCell,
  TableRow,
  TableHead,
  TableBody,
  Paper,
  CircularProgress,
  Button,
  Alert,
  IconButton,
  Collapse,
} from "@mui/material";
import {
  Delete as DeleteIcon,
  Edit as EditIcon,
} from "@mui/icons-material";
import { listProfiles, deleteProfile } from "@/queries/profiles";
import CreateProfileModal from "@/components/CreateProfileModal";
import EditProfileModal from "@/components/EditProfileModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


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
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

  if (!cookies.user_token) {
    router.push("/login")
  }

  const [profiles, setProfiles] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [editData, setEditData] = useState<ProfileData | null>(null);
  const [selectedProfile, setSelectedProfile] = useState<string | null>(null);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const fetchProfiles = async () => {
    setLoading(true);
    try {
      const data = await listProfiles(cookies.user_token);
      setProfiles(data);
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

  const handleModalSuccess = () => {
    fetchProfiles();
    setAlert({ message: "Profile created successfully!" });
  };

  const handleEditClick = (profile: any) => {
    const mappedProfile = {
      name: profile.name,
      ipPool: profile["ue-ip-pool"],
      dns: profile.dns,
      mtu: profile.mtu || 1500,
      bitrateUpValue: parseInt(profile["bitrate-uplink"]) || 100,
      bitrateUpUnit: profile["bitrate-uplink"].includes("Gbps") ? "Gbps" : "Mbps",
      bitrateDownValue: parseInt(profile["bitrate-downlink"]) || 100,
      bitrateDownUnit: profile["bitrate-downlink"].includes("Gbps") ? "Gbps" : "Mbps",
      fiveQi: profile["var5qi"] || 1,
      priorityLevel: profile["priority-level"] || 1,
    };

    setEditData(mappedProfile);
    setEditModalOpen(true);
  };

  const handleEditModalClose = () => {
    setEditModalOpen(false);
    setEditData(null);
  };

  const handleEditSuccess = () => {
    fetchProfiles();
    setAlert({ message: "Profile updated successfully!" });
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
        });
        fetchProfiles();
      } catch (error) {
        console.error("Error deleting profile:", error);
        setAlert({
          message: `Failed to delete profile "${selectedProfile}".`,
        });
      } finally {
        setSelectedProfile(null);
      }
    }
  };

  const handleConfirmationClose = () => {
    setConfirmationOpen(false);
    setSelectedProfile(null);
  };

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
            severity={"success"}
            onClose={() => setAlert({ message: "" })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>
      {!loading && profiles.length > 0 && (
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
            Profiles
          </Typography>
          <Button
            variant="contained"
            color="success"
            onClick={handleOpenCreateModal}
          >
            Create
          </Button>
        </Box>
      )}
      {loading ? (
        <Box
          sx={{
            height: "100vh",
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
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <Box
          sx={{
            width: "60%",
            overflowX: "auto",
          }}
        >
          <TableContainer component={Paper}>
            <Table sx={{ minWidth: 900 }} aria-label="profile table">
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell align="right">IP Pool</TableCell>
                  <TableCell align="right">DNS</TableCell>
                  <TableCell align="right">Bitrate (up)</TableCell>
                  <TableCell align="right">Bitrate (down)</TableCell>
                  <TableCell align="right">5QI</TableCell>
                  <TableCell align="right">Priority Level</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {profiles.map((profile) => (
                  <TableRow
                    key={profile.name}
                    sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
                  >
                    <TableCell component="th" scope="row">
                      {profile.name}
                    </TableCell>
                    <TableCell align="right">{profile?.["ue-ip-pool"]}</TableCell>
                    <TableCell align="right">{profile?.["dns"]}</TableCell>
                    <TableCell align="right">{profile?.["bitrate-uplink"]}</TableCell>
                    <TableCell align="right">{profile?.["bitrate-downlink"]}</TableCell>
                    <TableCell align="right">{profile?.["var5qi"]}</TableCell>
                    <TableCell align="right">{profile?.["priority-level"]}</TableCell>
                    <TableCell align="right">
                      <IconButton
                        aria-label="edit"
                        onClick={() => handleEditClick(profile)}
                      >
                        <EditIcon />
                      </IconButton>
                      <IconButton
                        aria-label="delete"
                        onClick={() => handleDeleteClick(profile.name)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}
      <CreateProfileModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={handleModalSuccess}
      />
      <EditProfileModal
        open={isEditModalOpen}
        onClose={handleEditModalClose}
        onSuccess={handleEditSuccess}
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
        onClose={handleConfirmationClose}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the profile "${selectedProfile}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default Profile;
