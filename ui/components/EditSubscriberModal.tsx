import React, { useState, useEffect } from "react";
import {
    Box,
    Modal,
    TextField,
    Button,
    Typography,
    Alert,
    Collapse,
    Select,
    MenuItem,
    InputLabel,
    FormControl,
} from "@mui/material";
import * as yup from "yup";
import { updateSubscriber } from "@/queries/subscribers";
import { listProfiles } from "@/queries/profiles";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface EditSubscriberModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        imsi: string;
        opc: string;
        key: string;
        sequenceNumber: string;
        profileName: string;
    };
}

const EditSubscriberModal: React.FC<EditSubscriberModalProps> = ({
    open,
    onClose,
    onSuccess,
    initialData,
}) => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

    if (!cookies.user_token) {
        router.push("/login")
    }
    const [formValues, setFormValues] = useState(initialData);
    const [profiles, setProfiles] = useState<string[]>([]);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        const fetchProfiles = async () => {
            try {
                const profileData = await listProfiles(cookies.user_token);
                setProfiles(profileData.map((profile: any) => profile.name));
            } catch (error) {
                console.error("Failed to fetch profiles:", error);
            }
        };

        if (open) {
            fetchProfiles();
            setFormValues(initialData);
            setErrors({});
        }
    }, [open, initialData]);

    const handleChange = (field: string, value: string | number) => {
        setFormValues((prev) => ({
            ...prev,
            [field]: value,
        }));
    };

    const handleSubmit = async () => {
        setLoading(true);
        setAlert({ message: "" });

        try {
            await updateSubscriber(
                cookies.user_token,
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
            setAlert({ message: `Failed to update subscriber: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="edit-subscriber-modal-title"
            aria-describedby="edit-subscriber-modal-description"
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
                <Typography id="edit-subscriber-modal-title" variant="h6" gutterBottom>
                    Edit Subscriber
                </Typography>
                <Collapse in={!!alert.message}>
                    <Alert
                        onClose={() => setAlert({ message: "" })}
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
                    margin="normal"
                    disabled
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
                <FormControl fullWidth margin="normal">
                    <InputLabel id="profile-name-label">Profile Name</InputLabel>
                    <Select
                        labelId="profile-name-label"
                        value={formValues.profileName}
                        onChange={(e) => handleChange("profileName", e.target.value)}
                        error={!!errors.profileName}
                    >
                        {profiles.map((profile) => (
                            <MenuItem key={profile} value={profile}>
                                {profile}
                            </MenuItem>
                        ))}
                    </Select>
                </FormControl>
                <Box sx={{ textAlign: "right", marginTop: 2 }}>
                    <Button onClick={onClose} sx={{ marginRight: 2 }}>
                        Cancel
                    </Button>
                    <Button
                        variant="contained"
                        color="success"
                        onClick={handleSubmit}
                        disabled={loading}
                    >
                        {loading ? "Updating..." : "Update"}
                    </Button>
                </Box>
            </Box>
        </Modal>
    );
};

export default EditSubscriberModal;
