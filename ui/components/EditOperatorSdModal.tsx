import React, { useState, useEffect } from "react";
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogContentText,
    DialogActions,
    TextField,
    Button,
    Alert,
    Collapse,
} from "@mui/material";
import * as yup from "yup";
import { updateOperatorSd } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorSdModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        sd: number;
    };
}

const schema = yup.object().shape({
    sd: yup
        .number()
        .required("SD is required")
        .integer("SD must be an integer")
        .min(0, "SD must be at least 0")
        .max(16777215, "SD must be at most 16777215"),
});

const EditOperatorSdModal: React.FC<EditOperatorSdModalProps> = ({
    open,
    onClose,
    onSuccess,
    initialData,
}) => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

    if (!cookies.user_token) {
        router.push("/login");
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

    const handleChange = (field: string, value: string) => {
        setFormValues((prev) => ({
            ...prev,
            [field]: value,
        }));

        setErrors((prev) => ({
            ...prev,
            [field]: "",
        }));
    };

    const validate = async (): Promise<boolean> => {
        try {
            await schema.validate(formValues, { abortEarly: false });
            setErrors({});
            return true;
        } catch (err: any) {
            const validationErrors: Record<string, string> = {};
            err.inner.forEach((error: yup.ValidationError) => {
                if (error.path) {
                    validationErrors[error.path] = error.message;
                }
            });
            setErrors(validationErrors);
            return false;
        }
    };

    const handleSubmit = async () => {
        const isValid = await validate();
        if (!isValid) return;

        setLoading(true);
        setAlert({ message: "" });

        try {
            await updateOperatorSd(cookies.user_token, formValues.sd);
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update operator SD: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-operator-sd-modal-title"
            aria-describedby="edit-operator-sd-modal-description"
        >
            <DialogTitle>Edit Operator SD</DialogTitle>
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
                <DialogContentText id="alert-dialog-slide-description">
                    The Service Differentiator (SD) is a 24-bit field that is used to differentiate slices.
                </DialogContentText>
                <TextField
                    fullWidth
                    label="SD"
                    value={formValues.sd}
                    onChange={(e) => handleChange("sd", e.target.value)}
                    error={!!errors.sd}
                    helperText={errors.sd}
                    margin="normal"
                />
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} >
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
        </Dialog >
    );
};

export default EditOperatorSdModal;
