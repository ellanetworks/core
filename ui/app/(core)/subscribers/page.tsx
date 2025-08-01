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
import {
  Delete as DeleteIcon,
  Edit as EditIcon,
  Visibility as VisibilityIcon,
} from "@mui/icons-material";
import { listSubscribers, deleteSubscriber } from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import ViewSubscriberModal from "@/components/ViewSubscriberModal";
import EditSubscriberModal from "@/components/EditSubscriberModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { Subscriber } from "@/types/types";

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

  const handleDeleteClick = (subscriberName: string) => {
    setSelectedSubscriber(subscriberName);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedSubscriber) {
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
    }
  };

  // Define base columns (common for all roles)
  const baseColumns: GridColDef[] = [
    { field: "imsi", headerName: "IMSI", flex: 1 },
    { field: "ipAddress", headerName: "IP Address", flex: 1 },
    { field: "policyName", headerName: "Policy", flex: 1 },
  ];

  if (role === "Admin" || role === "Network Manager") {
    baseColumns.push({
      field: "actions",
      headerName: "Actions",
      type: "actions",
      flex: 0.5,
      getActions: (params) => [
        <IconButton
          key="view"
          aria-label="view"
          onClick={() => handleViewClick(params.row)}
        >
          <VisibilityIcon />
        </IconButton>,
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
          onClick={() => handleDeleteClick(params.row.imsi)}
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
              marginBottom: 4,
              width: "60%",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography variant="h4" component="h1" gutterBottom>
              Subscribers ({subscribers.length})
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
              rows={subscribers}
              columns={baseColumns}
              getRowId={(row) => row.imsi}
              disableRowSelectionOnClick
            />
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
        initialData={
          editData || {
            imsi: "",
            policyName: "",
          }
        }
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
