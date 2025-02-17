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
import * as yup from "yup";
import { ValidationError } from "yup";
import { createUser } from "@/queries/users";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface CreateUserModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    email: yup.string().email().required("Email is required"),
    password: yup.string().min(1).max(256).required("Password is required"),
});

const CreateUserModal: React.FC<CreateUserModalProps> = ({ open, onClose, onSuccess }) => {
    const router = useRouter();
    const [cookies] = useCookies(['user_token']);

    if (!cookies.user_token) {
        router.push("/login");
    }

    const [formValues, setFormValues] = useState({
        email: "",
        role: 0, // 0 => Admin, 1 => Read Only
        password: "",
    });

    const [errors, setErrors] = useState<Record<string, string>>({});
    const [touched, setTouched] = useState<Record<string, boolean>>({});
    const [isValid, setIsValid] = useState(false);
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    const handleChange = (field: string, value: string | number) => {
        setFormValues((prev) => ({
            ...prev,
            [field]: value,
        }));
        validateField(field, value);
    };

    const handleBlur = (field: string) => {
        setTouched((prev) => ({
            ...prev,
            [field]: true,
        }));
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
            await createUser(
                cookies.user_token,
                formValues.email,
                formValues.role,
                formValues.password,
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({
                message: `Failed to create user: ${errorMessage}`,
            });
            console.error("Failed to create user:", error);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="create-user-modal-title"
            aria-describedby="create-user-modal-description"
        >
            <DialogTitle id="create-user-modal-title">Create User</DialogTitle>
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
                    onChange={(e) => handleChange("email", e.target.value)}
                    onBlur={() => handleBlur("email")}
                    error={!!errors.email && touched.email}
                    helperText={touched.email ? errors.email : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Password"
                    type="password"
                    value={formValues.password}
                    onChange={(e) => handleChange("password", e.target.value)}
                    onBlur={() => handleBlur("password")}
                    error={!!errors.password && touched.password}
                    helperText={touched.password ? errors.password : ""}
                    margin="normal"
                />
                <FormControl fullWidth margin="normal">
                    <InputLabel id="role-select-label">Role</InputLabel>
                    <Select
                        labelId="role-select-label"
                        id="role-select"
                        value={formValues.role}
                        label="Role"
                        onChange={(e) => handleChange("role", e.target.value as number)}
                    >
                        <MenuItem value={0}>Admin</MenuItem>
                        <MenuItem value={1}>Read Only</MenuItem>
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
                    disabled={!isValid || loading}
                >
                    {loading ? "Creating..." : "Create"}
                </Button>
            </DialogActions>
        </Dialog>
    );
};

export default CreateUserModal;
