import React, { useState, useEffect } from "react";
import {
    Box,
    Modal,
    TextField,
    Button,
    Typography,
    MenuItem,
    Alert,
    Collapse,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createProfile } from "@/queries/profiles";

interface CreateProfileModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    name: yup.string().min(1).max(256).required("Name is required"),
    ipPool: yup
        .string()
        .matches(
            /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\/\d{1,2}$/,
            "Must be a valid IP pool (e.g., 192.168.0.0/24)"
        )
        .required("IP Pool is required"),
    dns: yup
        .string()
        .matches(
            /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)$/,
            "Must be a valid IP address"
        )
        .required("DNS is required"),
    mtu: yup.number().min(1).max(65535).required("MTU is required"),
    bitrateUpValue: yup
        .number()
        .min(1, "Bitrate value must be between 1 and 999")
        .max(999, "Bitrate value must be between 1 and 999")
        .required("Bitrate value is required"),
    bitrateUpUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
    bitrateDownValue: yup
        .number()
        .min(1, "Bitrate value must be between 1 and 999")
        .max(999, "Bitrate value must be between 1 and 999")
        .required("Bitrate value is required"),
    bitrateDownUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
    fiveQi: yup.number().min(0).max(256).required("5QI is required"),
    priorityLevel: yup.number().min(0).max(256).required("Priority Level is required"),
});

const CreateProfileModal: React.FC<CreateProfileModalProps> = ({ open, onClose, onSuccess }) => {
    const [formValues, setFormValues] = useState({
        name: "",
        ipPool: "192.168.0.0/24",
        dns: "8.8.8.8",
        mtu: 1500,
        bitrateUpValue: 100,
        bitrateUpUnit: "Mbps",
        bitrateDownValue: 100,
        bitrateDownUnit: "Mbps",
        fiveQi: 1,
        priorityLevel: 1,
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
            const bitrateUplink = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
            const bitrateDownlink = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;
            await createProfile(
                formValues.name,
                formValues.ipPool,
                formValues.dns,
                formValues.mtu,
                bitrateUplink,
                bitrateDownlink,
                formValues.fiveQi,
                formValues.priorityLevel
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({
                message: `Failed to create profile: ${errorMessage}`,
            });
            console.error("Failed to create profile:", error);
        } finally {
            setLoading(false);
        }
    };


    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="create-profile-modal-title"
            aria-describedby="create-profile-modal-description"
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
                <Typography id="create-profile-modal-title" variant="h6" gutterBottom>
                    Create Profile
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
                    label="Name"
                    value={formValues.name}
                    onChange={(e) => handleChange("name", e.target.value)}
                    error={!!errors.name}
                    helperText={errors.name}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="IP Pool"
                    value={formValues.ipPool}
                    onChange={(e) => handleChange("ipPool", e.target.value)}
                    error={!!errors.ipPool}
                    helperText={errors.ipPool}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="DNS"
                    value={formValues.dns}
                    onChange={(e) => handleChange("dns", e.target.value)}
                    error={!!errors.dns}
                    helperText={errors.dns}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="MTU"
                    type="number"
                    value={formValues.mtu}
                    onChange={(e) => handleChange("mtu", Number(e.target.value))}
                    error={!!errors.mtu}
                    helperText={errors.mtu}
                    margin="normal"
                />
                <Box display="flex" gap={2}>
                    <TextField
                        label="Bitrate Up"
                        type="number"
                        value={formValues.bitrateUpValue}
                        onChange={(e) => handleChange("bitrateUpValue", Number(e.target.value))}
                        error={!!errors.bitrateUpValue}
                        helperText={errors.bitrateUpValue}
                        margin="normal"
                    />
                    <TextField
                        select
                        label="Unit"
                        value={formValues.bitrateUpUnit}
                        onChange={(e) => handleChange("bitrateUpUnit", e.target.value)}
                        margin="normal"
                    >
                        <MenuItem value="Mbps">Mbps</MenuItem>
                        <MenuItem value="Gbps">Gbps</MenuItem>
                    </TextField>
                </Box>
                <Box display="flex" gap={2}>
                    <TextField
                        label="Bitrate Down"
                        type="number"
                        value={formValues.bitrateDownValue}
                        onChange={(e) => handleChange("bitrateDownValue", Number(e.target.value))}
                        error={!!errors.bitrateDownValue}
                        helperText={errors.bitrateDownValue}
                        margin="normal"
                    />
                    <TextField
                        select
                        label="Unit"
                        value={formValues.bitrateDownUnit}
                        onChange={(e) => handleChange("bitrateDownUnit", e.target.value)}
                        margin="normal"
                    >
                        <MenuItem value="Mbps">Mbps</MenuItem>
                        <MenuItem value="Gbps">Gbps</MenuItem>
                    </TextField>
                </Box>
                <TextField
                    fullWidth
                    label="5QI"
                    type="number"
                    value={formValues.fiveQi}
                    onChange={(e) => handleChange("fiveQi", Number(e.target.value))}
                    error={!!errors.fiveQi}
                    helperText={errors.fiveQi}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Priority Level"
                    type="number"
                    value={formValues.priorityLevel}
                    onChange={(e) => handleChange("priorityLevel", Number(e.target.value))}
                    error={!!errors.priorityLevel}
                    helperText={errors.priorityLevel}
                    margin="normal"
                />
                <Box sx={{ textAlign: "right", marginTop: 2 }}>
                    <Button onClick={onClose} sx={{ marginRight: 2 }}>
                        Cancel
                    </Button>
                    <Button
                        variant="contained"
                        color="primary"
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

export default CreateProfileModal;
