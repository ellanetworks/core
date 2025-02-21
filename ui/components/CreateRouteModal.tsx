import React, { useState, useEffect } from "react";
import {
    Box,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    TextField,
    Button,
    Alert,
    Collapse,
    MenuItem,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createRoute } from "@/queries/routes";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface CreateRouteModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    destination: yup.string().min(1).max(256).required("Destination is required"),
    gateway: yup.string().min(1).max(256).required("Gateway is required"),
    interface: yup.string().min(1).max(256).required("Interface is required"),
    metric: yup.number().required("Metric is required"),
});

const CreateRouteModal: React.FC<CreateRouteModalProps> = ({ open, onClose, onSuccess }) => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

    if (!cookies.user_token) {
        router.push("/login")
    }

    const [formValues, setFormValues] = useState({
        destination: "",
        gateway: "",
        interface: "",
        metric: 0,
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
            await createRoute(
                cookies.user_token,
                formValues.destination,
                formValues.gateway,
                formValues.interface,
                formValues.metric,
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({
                message: `Failed to create route: ${errorMessage}`,
            });
            console.error("Failed to create route:", error);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="create-route-modal-title"
            aria-describedby="create-route-modal-description"
        >
            <DialogTitle>Create Route</DialogTitle>
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
                    label="Destination"
                    value={formValues.destination}
                    onChange={(e) => handleChange("destination", e.target.value)}
                    onBlur={() => handleBlur("destination")}
                    error={!!errors.destination && touched.destination}
                    helperText={touched.destination ? errors.destination : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Gateway"
                    value={formValues.gateway}
                    onChange={(e) => handleChange("gateway", e.target.value)}
                    onBlur={() => handleBlur("gateway")}
                    error={!!errors.gateway && touched.gateway}
                    helperText={touched.gateway ? errors.gateway : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Interface"
                    value={formValues.interface}
                    onChange={(e) => handleChange("interface", e.target.value)}
                    onBlur={() => handleBlur("interface")}
                    error={!!errors.interface && touched.interface}
                    helperText={touched.interface ? errors.interface : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Metric"
                    type="number"
                    value={formValues.metric}
                    onChange={(e) => handleChange("metric", Number(e.target.value))}
                    onBlur={() => handleBlur("metric")}
                    error={!!errors.metric && touched.metric}
                    helperText={touched.metric ? errors.metric : ""}
                    margin="normal"
                />
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

export default CreateRouteModal;
