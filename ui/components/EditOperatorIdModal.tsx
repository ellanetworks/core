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
} from "@mui/material";
import * as yup from "yup";
import { updateOperatorId } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorIdModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        mcc: string;
        mnc: string;
    };
}

const schema = yup.object().shape({
    mcc: yup
        .string()
        .matches(/^\d{3}$/, "MCC must be a 3 decimal digit")
        .required("MCC is required"),
    mnc: yup
        .string()
        .matches(/^\d{2,3}$/, "MNC must be a 2 or 3 decimal digit")
        .required("MNC is required"),
});

const EditOperatorIdModal: React.FC<EditOperatorIdModalProps> = ({
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

        // Reset the field's error when the user types
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
            await updateOperatorId(cookies.user_token, formValues.mcc, formValues.mnc);
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update operator ID: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-operator-id-modal-title"
            aria-describedby="edit-operator-id-modal-description"
        >
            <DialogTitle>Edit Operator ID</DialogTitle>
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
                    label="MCC"
                    value={formValues.mcc}
                    onChange={(e) => handleChange("mcc", e.target.value)}
                    error={!!errors.mcc}
                    helperText={errors.mcc}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="MNC"
                    value={formValues.mnc}
                    onChange={(e) => handleChange("mnc", e.target.value)}
                    error={!!errors.mnc}
                    helperText={errors.mnc}
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

export default EditOperatorIdModal;
