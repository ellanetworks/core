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
import { updateOperatorSst } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorSstModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        sst: number;
    };
}

const schema = yup.object().shape({
    sst: yup
        .number()
        .required("SST is required")
        .integer("SST must be an integer")
        .min(0, "SST must be at least 0")
        .max(255, "SST must be at most 255"),
});

const EditOperatorIdModal: React.FC<EditOperatorSstModalProps> = ({
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
            await updateOperatorSst(cookies.user_token, formValues.sst);
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update operator SST: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-operator-sst-modal-title"
            aria-describedby="edit-operator-sst-modal-description"
        >
            <DialogTitle>Edit Operator SST</DialogTitle>
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
                    The Slice Service Type (SST) is a 8-bit field that identifies the type of service provided by the slice.
                </DialogContentText>
                <TextField
                    fullWidth
                    label="SST"
                    value={formValues.sst}
                    onChange={(e) => handleChange("sst", e.target.value)}
                    error={!!errors.sst}
                    helperText={errors.sst}
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
