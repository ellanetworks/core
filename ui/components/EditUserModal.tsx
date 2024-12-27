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
import { updateUser } from "@/queries/users";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditUserModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        username: string;
        // We only have `username` here, no password from the server/props
    };
}

// 1. Define form values interface
interface FormValues {
    username: string;
    password: string;
}

const EditUserModal: React.FC<EditUserModalProps> = ({
    open,
    onClose,
    onSuccess,
    initialData,
}) => {
    const router = useRouter();
    const [cookies] = useCookies(["user_token"]);

    if (!cookies.user_token) {
        router.push("/login");
    }

    // 2. Use the interface here
    const [formValues, setFormValues] = useState<FormValues>({
        username: initialData.username,
        password: "",
    });

    const [errors, setErrors] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        if (open) {
            // Whenever modal opens, reset the form:
            setFormValues({
                username: initialData.username,
                password: "",
            });
            setErrors({});
        }
    }, [open, initialData]);

    const handleChange = (field: keyof FormValues, value: string) => {
        setFormValues((prev) => ({
            ...prev,
            [field]: value,
        }));
    };

    const handleSubmit = async () => {
        setLoading(true);
        setAlert({ message: "" });

        try {
            await updateUser(
                cookies.user_token,
                formValues.username,
                formValues.password
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update user: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="edit-user-modal-title"
            aria-describedby="edit-user-modal-description"
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
                <Typography id="edit-user-modal-title" variant="h6" gutterBottom>
                    Edit User
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
                    label="Username"
                    value={formValues.username}
                    margin="normal"
                    disabled
                />
                <TextField
                    fullWidth
                    label="Password"
                    type="password"
                    value={formValues.password}
                    onChange={(e) => handleChange("password", e.target.value)}
                    error={!!errors.password}
                    helperText={errors.password}
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

export default EditUserModal;
