import React, { useState, useEffect } from "react";
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    TextField,
    Button,
    Alert,
    Collapse,
    Select,
    MenuItem,
    InputLabel,
    FormControl,
} from "@mui/material";
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
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-subscriber-modal-title"
            aria-describedby="edit-subscriber-modal-description"
        >
            <DialogTitle>Edit Subscriber</DialogTitle>
            <DialogContent dividers>
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
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>
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
            </DialogActions>
        </Dialog>
    );
};

export default EditSubscriberModal;
