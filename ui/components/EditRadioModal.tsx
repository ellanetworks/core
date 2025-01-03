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
import { updateRadio } from "@/queries/radios";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface EditRadioModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        name: string;
        tac: string;
    };
}

const EditRadioModal: React.FC<EditRadioModalProps> = ({
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
            await updateRadio(
                cookies.user_token,
                formValues.name,
                formValues.tac,
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update radio: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-radio-modal-title"
            aria-describedby="edit-radio-modal-description"
        >
            <DialogTitle>Edit Radio</DialogTitle>
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
                    label="Name"
                    value={formValues.name}
                    margin="normal"
                    disabled
                />
                <TextField
                    fullWidth
                    label="TAC"
                    value={formValues.tac}
                    onChange={(e) => handleChange("tac", e.target.value)}
                    error={!!errors.tac}
                    helperText={errors.tac}
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

export default EditRadioModal;
