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
    Visibility as VisibilityIcon,
} from "@mui/icons-material";
import { listSubscribers, deleteSubscriber } from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import ViewSubscriberModal from "@/components/ViewSubscriberModal";
import EditSubscriberModal from "@/components/EditSubscriberModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface SubscriberData {
    imsi: string;
    ipAddress: string;
    profileName: string;
}

const Subscriber = () => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

    if (!cookies.user_token) {
        router.push("/login");
    }

    const [subscribers, setSubscribers] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [isCreateModalOpen, setCreateModalOpen] = useState(false);
    const [isConfirmationOpen, setConfirmationOpen] = useState(false);
    const [isEditModalOpen, setEditModalOpen] = useState(false);
    const [isViewModalOpen, setViewModalOpen] = useState(false);
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
    const handleOpenViewModal = () => setViewModalOpen(true);
    const handleCloseViewModal = () => {
        setSelectedSubscriber(null);
        setViewModalOpen(false);
    };

    const handleCreateModalSuccess = () => {
        fetchSubscribers();
        setAlert({ message: "Subscriber created successfully!" });
    };

    const handleEditClick = (subscriber: any) => {
        const mappedSubscriber = {
            imsi: subscriber.imsi,
            ipAddress: subscriber.ipAddress,
            opc: subscriber.opc,
            key: subscriber.key,
            sequenceNumber: subscriber.sequenceNumber,
            profileName: subscriber.profileName,
        };

        setEditData(mappedSubscriber);
        setEditModalOpen(true);
    };

    const handleViewClick = (subscriber: any) => {
        setSelectedSubscriber(subscriber.imsi); // Set selected IMSI
        setViewModalOpen(true); // Open the modal
    };

    const handleEditModalClose = () => {
        setEditModalOpen(false);
        setEditData(null);
    };

    const handleEditSuccess = () => {
        fetchSubscribers();
        setAlert({ message: "Subscribers updated successfully!" });
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

    const handleConfirmationClose = () => {
        setConfirmationOpen(false);
        setSelectedSubscriber(null);
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
            {!loading && subscribers.length > 0 && (
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
                        Subscribers
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
            ) : subscribers.length === 0 ? (
                <EmptyState
                    primaryText="No subscriber found."
                    secondaryText="Create a new subscriber."
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
                        <Table sx={{ minWidth: 900 }} aria-label="subscriber table">
                            <TableHead>
                                <TableRow>
                                    <TableCell>IMSI</TableCell>
                                    <TableCell align="right">IP Address</TableCell>
                                    <TableCell align="right">Profile</TableCell>
                                    <TableCell align="right">Actions</TableCell>
                                </TableRow>
                            </TableHead>
                            <TableBody>
                                {subscribers.map((subscriber) => (
                                    <TableRow
                                        key={subscriber.imsi}
                                        sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
                                    >
                                        <TableCell component="th" scope="row">
                                            {subscriber.imsi}
                                        </TableCell>
                                        <TableCell align="right">{subscriber.ipAddress}</TableCell>
                                        <TableCell align="right">{subscriber.profileName}</TableCell>
                                        <TableCell align="right">
                                            <IconButton
                                                aria-label="view"
                                                onClick={() => handleViewClick(subscriber)}
                                            >
                                                <VisibilityIcon />
                                            </IconButton>
                                            <IconButton
                                                aria-label="edit"
                                                onClick={() => handleEditClick(subscriber)}
                                            >
                                                <EditIcon />
                                            </IconButton>
                                            <IconButton
                                                aria-label="delete"
                                                onClick={() => handleDeleteClick(subscriber.imsi)}
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
            <ViewSubscriberModal
                open={isViewModalOpen}
                onClose={handleCloseViewModal}
                imsi={selectedSubscriber || ""} // Pass the selected IMSI
            />
            <CreateSubscriberModal
                open={isCreateModalOpen}
                onClose={handleCloseCreateModal}
                onSuccess={handleCreateModalSuccess}
            />
            <EditSubscriberModal
                open={isEditModalOpen}
                onClose={handleEditModalClose}
                onSuccess={handleEditSuccess}
                initialData={
                    editData || {
                        imsi: "",
                        profileName: "",
                    }
                }
            />
            <DeleteConfirmationModal
                open={isConfirmationOpen}
                onClose={handleConfirmationClose}
                onConfirm={handleDeleteConfirm}
                title="Confirm Deletion"
                description={`Are you sure you want to delete the subscriber "${selectedSubscriber}"? This action cannot be undone.`}
            />
        </Box>
    );
};

export default Subscriber;
