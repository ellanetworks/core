"use client";

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
    Autocomplete,
    Chip,
} from "@mui/material";
import * as yup from "yup";
import { updateOperatorTracking } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorTrackingModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
    initialData: {
        supportedTacs: string[];
    };
}

const schema = yup.string().matches(/^\d{3}$/, "Each TAC must be a 3-digit number");

const EditOperatorTrackingModal: React.FC<EditOperatorTrackingModalProps> = ({
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

    const [formValues, setFormValues] = useState<{ supportedTacs: string[] }>({ supportedTacs: [] });
    const [errors, setErrors] = useState<{ supportedTacs?: string }>({});
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        if (open) {
            setFormValues(initialData);
            setErrors({});
        }
    }, [open, initialData]);

    const validateTacs = (tacs: string[]): boolean => {
        const invalidTacs = tacs.filter((tac) => !schema.isValidSync(tac));
        if (invalidTacs.length > 0) {
            setErrors({ supportedTacs: `Invalid TACs: ${invalidTacs.join(", ")}` });
            return false;
        }
        setErrors({});
        return true;
    };

    const handleTacsChange = (event: any, value: string[]) => {
        setFormValues({ supportedTacs: value });
        validateTacs(value);
    };

    const handleSubmit = async () => {
        if (!validateTacs(formValues.supportedTacs)) return;

        setLoading(true);
        setAlert({ message: "" });

        try {
            await updateOperatorTracking(cookies.user_token, formValues.supportedTacs);
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({ message: `Failed to update supported TACs: ${errorMessage}` });
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="edit-operator-tracking-modal-title"
            aria-describedby="edit-operator-tracking-modal-description"
        >
            <DialogTitle>Edit Operator Tracking Information</DialogTitle>
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
                <DialogContentText id="edit-operator-supportedtacs-modal-description" sx={{ marginBottom: 3 }}>
                    Tracking Area Codes (TACs) are used to identify a tracking area in a mobile network. Only radios with TACs listed here will be able to connect to the network.
                </DialogContentText>
                <Autocomplete
                    multiple
                    freeSolo
                    options={[]}
                    value={formValues.supportedTacs}
                    onChange={handleTacsChange}
                    renderTags={(value: readonly string[], getTagProps) =>
                        value.map((option: string, index: number) => (
                            <Chip
                                variant="outlined"
                                label={option}
                                {...getTagProps({ index })}
                            />
                        ))
                    }
                    renderInput={(params) => (
                        <TextField
                            {...params}
                            variant="outlined"
                            label="Supported TACs"
                            placeholder="Enter TACs (e.g., 001)"
                            error={!!errors.supportedTacs}
                            helperText={errors.supportedTacs || "Enter each TAC as a 3-digit number"}
                        />
                    )}
                    sx={{ marginBottom: 2 }}
                />
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} disabled={loading}>
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
        </Dialog>
    );
};

export default EditOperatorTrackingModal;
