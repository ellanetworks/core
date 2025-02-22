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
    FormControl,
    InputLabel,
    Select,
    MenuItem,
} from "@mui/material";
import { updateUser } from "@/queries/users";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditUserModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        email: string;
        role: string;
    };
}

interface FormValues {
    email: string;
    role: string;
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

    const [formValues, setFormValues] = useState<FormValues>({
        email: "",
        role: "",
    });

    const [errors, setErrors] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        if (open) {
            const convertedRole =
                initialData.role === "Admin" ? "admin"
                    : initialData.role === "Read Only" ? "readonly"
                        : initialData.role === "Network Manager" ? "network-manager"
                            : "";
            setFormValues({
                email: initialData.email,
                role: convertedRole,
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
                formValues.email,
                formValues.role
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
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-user-modal-title"
            aria-describedby="edit-user-modal-description"
        >
            <DialogTitle>Edit User</DialogTitle>
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
                    label="Email"
                    value={formValues.email}
                    margin="normal"
                    disabled
                />
                <FormControl fullWidth margin="normal">
                    <InputLabel id="role-select-label">Role</InputLabel>
                    <Select
                        labelId="role-select-label"
                        id="role-select"
                        value={formValues.role}
                        label="Role"
                        onChange={(e) => handleChange("role", e.target.value as string)}
                    >
                        <MenuItem value={"admin"}>Admin</MenuItem>
                        <MenuItem value={"network-manager"}>Network Manager</MenuItem>
                        <MenuItem value={"readonly"}>Read Only</MenuItem>
                    </Select>
                </FormControl>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>Cancel</Button>
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

export default EditUserModal;
