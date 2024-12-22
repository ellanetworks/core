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
import { listSubscribers, deleteSubscriber } from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import EditSubscriberModal from "@/components/EditSubscriberModal";
import DeleteSubscriberModal from "@/components/DeleteSubscriberModal";

interface SubscriberData {
    imsi: string;
    opc: string;
    key: string;
    sequenceNumber: string;
    profileName: string;
}

const Subscriber = () => {
    const [subscribers, setSubscribers] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [isCreateModalOpen, setCreateModalOpen] = useState(false);
    const [isConfirmationOpen, setConfirmationOpen] = useState(false);

    const [isEditModalOpen, setEditModalOpen] = useState(false);
    const [editData, setEditData] = useState<SubscriberData | null>(null);

    const handleEditClick = (subscriber: any) => {
        const mappedSubscriber = {
            imsi: subscriber.imsi,
            opc: subscriber.opc,
            key: subscriber.key,
            sequenceNumber: subscriber["sequenceNumber"],
            profileName: subscriber["profileName"],
        };

        setEditData(mappedSubscriber);
        setEditModalOpen(true);
    };


    const handleEditModalClose = () => {
        setEditModalOpen(false);
        setEditData(null);
    };

    const handleEditSuccess = () => {
        fetchSubscribers();
        setAlert({ message: "Subscriber updated successfully!" });
    };


    const [selectedSubscriber, setSelectedSubscriber] = useState<string | null>(null);
    const [alert, setAlert] = useState<{ message: string; }>({
        message: "",
    });

    const fetchSubscribers = async () => {
        setLoading(true);
        try {
            const data = await listSubscribers();
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

    const handleModalSuccess = () => {
        fetchSubscribers();
        setAlert({ message: "Subscriber created successfully!", });
    };

    const handleDeleteClick = (subscriberName: string) => {
        setSelectedSubscriber(subscriberName);
        setConfirmationOpen(true);
    };

    const handleDeleteConfirm = async () => {
        setConfirmationOpen(false);
        if (selectedSubscriber) {
            try {
                await deleteSubscriber(selectedSubscriber);
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
            <Box sx={{ width: "50%" }}>
                <Collapse in={!!alert.message}>
                    <Alert
                        severity={"success"}
                        onClose={() => setAlert({ message: "", })}
                        sx={{ marginBottom: 2 }}
                    >
                        {alert.message}
                    </Alert>
                </Collapse>
            </Box>
            <Box
                sx={{
                    marginBottom: 4,
                    width: "50%",
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                }}
            >
                <Typography variant="h4" component="h1" gutterBottom>
                    Subscribers
                </Typography>
                <Button variant="contained" color="primary" onClick={handleOpenCreateModal}>
                    Create
                </Button>
            </Box>
            <Box
                sx={{
                    width: "50%",
                    overflowX: "auto",
                }}
            >
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
                ) : (
                    <TableContainer component={Paper}>
                        <Table sx={{ minWidth: 900 }} aria-label="subscriber table">
                            <TableHead>
                                <TableRow>
                                    <TableCell>IMSI</TableCell>
                                    <TableCell align="right">OPC</TableCell>
                                    <TableCell align="right">Key</TableCell>
                                    <TableCell align="right">Sequence Number</TableCell>
                                    <TableCell align="right">Profile Name</TableCell>
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
                                        <TableCell align="right">{subscriber.opc}</TableCell>
                                        <TableCell align="right">{subscriber.key}</TableCell>
                                        <TableCell align="right">{subscriber["sequenceNumber"]}</TableCell>
                                        <TableCell align="right">{subscriber["profileName"]}</TableCell>
                                        <TableCell align="right">
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
                )}
            </Box>

            <CreateSubscriberModal
                open={isCreateModalOpen}
                onClose={handleCloseCreateModal}
                onSuccess={handleModalSuccess}
            />
            <EditSubscriberModal
                open={isEditModalOpen}
                onClose={handleEditModalClose}
                onSuccess={handleEditSuccess}
                initialData={
                    editData || {
                        imsi: "",
                        opc: "",
                        key: "",
                        sequenceNumber: "",
                        profileName: "",
                    }
                }
            />
            <DeleteSubscriberModal
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
