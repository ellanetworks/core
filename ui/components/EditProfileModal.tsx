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
import { updateProfile } from "@/queries/profiles";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface EditProfileModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
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
    };
}


const EditProfileModal: React.FC<EditProfileModalProps> = ({
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
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        if (open) {
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
            const bitrateUplink = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
            const bitrateDownlink = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;
            await updateProfile(
                cookies.user_token,
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
            setAlert({ message: `Failed to update profile: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="edit-profile-modal-title"
            aria-describedby="edit-profile-modal-description"
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
                <Typography id="edit-profile-modal-title" variant="h6" gutterBottom>
                    Edit Profile
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
                    label="Name"
                    value={formValues.name}
                    margin="normal"
                    disabled
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
                        label="Bitrate Up Value"
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
                        label="Bitrate Down Value"
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

export default EditProfileModal;
