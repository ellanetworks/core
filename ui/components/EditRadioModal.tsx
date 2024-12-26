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
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="edit-radio-modal-title"
            aria-describedby="edit-radio-modal-description"
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
                <Typography id="edit-radio-modal-title" variant="h6" gutterBottom>
                    Edit Radio
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
                    label="TAC"
                    value={formValues.tac}
                    onChange={(e) => handleChange("tac", e.target.value)}
                    error={!!errors.tac}
                    helperText={errors.tac}
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

export default EditRadioModal;
