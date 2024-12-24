import React, { useState, useEffect } from "react";
import {
    Box,
    Modal,
    TextField,
    Button,
    Typography,
    Alert,
    Collapse,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createSubscriber } from "@/queries/subscribers";

interface CreateSubscriberModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    imsi: yup.string().min(1).max(256).required("IMSI is required"),
    opc: yup.string().min(1).max(256).required("OPC is required"),
    key: yup.string().min(1).max(256).required("Key is required"),
    sequenceNumber: yup.string().min(1).max(256).required("Sequence Number is required"),
    profileName: yup.string().min(1).max(256).required("Profile Name is required"),
});

const CreateSubscriberModal: React.FC<CreateSubscriberModalProps> = ({ open, onClose, onSuccess }) => {
    const [formValues, setFormValues] = useState({
        imsi: "",
        opc: "",
        key: "",
        sequenceNumber: "",
        profileName: "",
    });

    const [errors, setErrors] = useState<Record<string, string>>({});
    const [isValid, setIsValid] = useState(false);
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string; }>({
        message: "",
    });

    const handleChange = (field: string, value: string | number) => {
        setFormValues((prev) => ({
            ...prev,
            [field]: value,
        }));
        validateField(field, value);
    };

    const validateField = async (field: string, value: string | number) => {
        try {
            const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
            await fieldSchema.validate(value);
            setErrors((prev) => ({
                ...prev,
                [field]: "",
            }));
        } catch (err) {
            if (err instanceof ValidationError) {
                setErrors((prev) => ({
                    ...prev,
                    [field]: err.message,
                }));
            }
        }
    };

    const validateForm = async () => {
        try {
            await schema.validate(formValues, { abortEarly: false });
            setErrors({});
            setIsValid(true);
        } catch (err) {
            if (err instanceof ValidationError) {
                const validationErrors = err.inner.reduce((acc, curr) => {
                    acc[curr.path!] = curr.message;
                    return acc;
                }, {} as Record<string, string>);
                setErrors(validationErrors);
            }
            setIsValid(false);
        }
    };

    useEffect(() => {
        validateForm();
    }, [formValues]);

    const handleSubmit = async () => {
        setLoading(true);
        setAlert({ message: "" });
        try {
            await createSubscriber(
                formValues.imsi,
                formValues.opc,
                formValues.key,
                formValues.sequenceNumber,
                formValues.profileName
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({
                message: `Failed to create subscriber: ${errorMessage}`,
            });
            console.error("Failed to create subscriber:", error);
        } finally {
            setLoading(false);
        }
    };


    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="create-subscriber-modal-title"
            aria-describedby="create-subscriber-modal-description"
        >
            <Box
                sx={{
                    position: "absolute",
                    top: "50%",
                    left: "50%",
                    transform: "translate(-50%, -50%)",
                    width: 600,
                    bgcolor: "background.paper",
                    border: "2px solid #000",
                    boxShadow: 24,
                    p: 4,
                }}
            >
                <Typography id="create-subscriber-modal-title" variant="h6" gutterBottom>
                    Create Subscriber
                </Typography>
                <Collapse in={!!alert.message}>
                    <Alert
                        onClose={() => setAlert({ message: "", })}
                        sx={{ mb: 2 }}
                        severity="error"
                    >
                        {alert.message}
                    </Alert>
                </Collapse>
                <TextField
                    fullWidth
                    label="IMSI"
                    value={formValues.imsi}
                    onChange={(e) => handleChange("imsi", e.target.value)}
                    error={!!errors.imsi}
                    helperText={errors.imsi}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="OPC"
                    value={formValues.opc}
                    onChange={(e) => handleChange("opc", e.target.value)}
                    error={!!errors.opc}
                    helperText={errors.opc}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Key"
                    value={formValues.key}
                    onChange={(e) => handleChange("key", e.target.value)}
                    error={!!errors.key}
                    helperText={errors.key}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Sequence Number"
                    value={formValues.sequenceNumber}
                    onChange={(e) => handleChange("sequenceNumber", e.target.value)}
                    error={!!errors.sequenceNumber}
                    helperText={errors.sequenceNumber}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Profile Name"
                    value={formValues.profileName}
                    onChange={(e) => handleChange("profileName", e.target.value)}
                    error={!!errors.profileName}
                    helperText={errors.profileName}
                    margin="normal"
                />
                <Box sx={{ textAlign: "right", marginTop: 2 }}>
                    <Button onClick={onClose} sx={{ marginRight: 2 }}>
                        Cancel
                    </Button>
                    <Button
                        variant="contained"
                        color="success"
                        onClick={handleSubmit}
                        disabled={!isValid || loading}
                    >
                        {loading ? "Creating..." : "Create"}
                    </Button>
                </Box>
            </Box>
        </Modal>
    );
};

export default CreateSubscriberModal;
