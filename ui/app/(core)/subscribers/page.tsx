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
import {
    DataGrid,
    GridColDef,
} from "@mui/x-data-grid";
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

interface SubscriberData {
    imsi: string;
    ipAddress: string;
    profileName: string;
}

const Subscriber = () => {
    const [cookies] = useCookies(["user_token"]);
    const [subscribers, setSubscribers] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [isCreateModalOpen, setCreateModalOpen] = useState(false);
    const [isEditModalOpen, setEditModalOpen] = useState(false);
    const [isViewModalOpen, setViewModalOpen] = useState(false);
    const [isConfirmationOpen, setConfirmationOpen] = useState(false);
    const [editData, setEditData] = useState<SubscriberData | null>(null);
    const [selectedSubscriber, setSelectedSubscriber] = useState<string | null>(null);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    const fetchSubscribers = async () => {
        setLoading(true);
        try {
            const data = await listSubscribers(cookies.user_token);
            setSubscribers(data);
        } catch (error) {
            console.error("Error fetching subscribers:", error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchSubscribers();
    }, []);

    const handleOpenCreateModal = () => setCreateModalOpen(true);
    const handleCloseCreateModal = () => setCreateModalOpen(false);
    const handleCloseViewModal = () => {
        setSelectedSubscriber(null);
        setViewModalOpen(false);
    };

    const handleEditClick = (subscriber: any) => {
        const mappedSubscriber = {
            imsi: subscriber.imsi,
            ipAddress: subscriber.ipAddress,
            profileName: subscriber.profileName,
        };
        setEditData(mappedSubscriber);
        setEditModalOpen(true);
    };

    const handleViewClick = (subscriber: any) => {
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
                });
                fetchSubscribers();
            } catch (error) {
                console.error("Error deleting subscriber:", error);
                setAlert({
                    message: `Failed to delete subscriber "${selectedSubscriber}".`,
                });
            } finally {
                setSelectedSubscriber(null);
            }
        }
    };

    const columns: GridColDef[] = [
        { field: "imsi", headerName: "IMSI", flex: 1 },
        { field: "ipAddress", headerName: "IP Address", flex: 1 },
        { field: "profileName", headerName: "Profile", flex: 1 },
        {
            field: "actions",
            headerName: "Actions",
            type: "actions",
            flex: 0.5,
            getActions: (params) => [
                <IconButton
                    aria-label="view"
                    onClick={() => handleViewClick(params.row)}
                >
                    <VisibilityIcon />
                </IconButton>,
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
                        severity="success"
                        onClose={() => setAlert({ message: "" })}
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
            ) : subscribers.length === 0 ? (
                <EmptyState
                    primaryText="No subscriber found."
                    secondaryText="Create a new subscriber."
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
                            Subscribers ({subscribers.length})
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
                            rows={subscribers}
                            columns={columns}
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
                        profileName: "",
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

export default Subscriber;
