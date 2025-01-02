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
import * as yup from "yup";
import { updateOperatorCode } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorCodeModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    operatorCode: yup
        .string()
        .required("Operator Code is required.")
        .matches(
            /^[0-9A-Fa-f]{32}$/,
            "Operator Code must be a 32-character hexadecimal string."
        ),
});

const EditOperatorCodeModal: React.FC<EditOperatorCodeModalProps> = ({
    open,
    onClose,
    onSuccess,
}) => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

    if (!cookies.user_token) {
        router.push("/login");
    }

    const [formValues, setFormValues] = useState<{ operatorCode: string }>({
        operatorCode: "",
    });
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        if (open) {
            setFormValues({ operatorCode: "" });
            setErrors({});
        }
    }, [open]);

    const handleChange = (field: string, value: string) => {
        setFormValues((prev) => ({
            ...prev,
            [field]: value,
        }));

        // Reset error when the user types
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
        if (!isValid) {
            return;
        }

        setLoading(true);
        setAlert({ message: "" });

        try {
            await updateOperatorCode(cookies.user_token, formValues.operatorCode);
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update operator code: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="edit-operator-code-modal-title"
            aria-describedby="edit-operator-code-modal-description"
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
                <Typography id="edit-operator-code-modal-title" variant="h6" gutterBottom>
                    Edit Operator Code
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
                    label="Operator Code"
                    value={formValues.operatorCode}
                    onChange={(e) => handleChange("operatorCode", e.target.value)}
                    error={!!errors.operatorCode}
                    helperText={errors.operatorCode}
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
                        disabled={loading || !formValues.operatorCode}
                    >
                        {loading ? "Updating..." : "Update"}
                    </Button>
                </Box>
            </Box>
        </Modal>
    );
};

export default EditOperatorCodeModal;
