"use client";

import React from "react";
import {
    Box,
    Typography,
    Button,
    Modal,
} from "@mui/material";

interface ConfirmationModalProps {
    open: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    description: string;
}

const DeleteSubscriberModal: React.FC<ConfirmationModalProps> = ({
    open,
    onClose,
    onConfirm,
    title,
    description,
}) => {
    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="confirmation-modal-title"
            aria-describedby="confirmation-modal-description"
        >
            <Box
                sx={{
                    position: "absolute",
                    top: "50%",
                    left: "50%",
                    transform: "translate(-50%, -50%)",
                    width: 400,
                    bgcolor: "background.paper",
                    border: "2px solid #000",
                    boxShadow: 24,
                    p: 4,
                }}
            >
                <Typography id="confirmation-modal-title" variant="h6" gutterBottom>
                    {title}
                </Typography>
                <Typography id="confirmation-modal-description" sx={{ mb: 3 }}>
                    {description}
                </Typography>
                <Box sx={{ textAlign: "right" }}>
                    <Button onClick={onClose} sx={{ marginRight: 2 }}>
                        Cancel
                    </Button>
                    <Button variant="contained" color="error" onClick={onConfirm}>
                        Confirm
                    </Button>
                </Box>
            </Box>
        </Modal>
    );
};

export default DeleteSubscriberModal;
