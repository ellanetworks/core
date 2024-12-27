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
import { ValidationError } from "yup";
import { createProfile } from "@/queries/profiles";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface CreateProfileModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    name: yup.string().min(1).max(256).required("Name is required"),
    ipPool: yup
        .string()
        .matches(
            /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\/\d{1,2}$/,
            "Must be a valid IP pool (e.g., 192.168.0.0/24)"
        )
        .required("IP Pool is required"),
    dns: yup
        .string()
        .matches(
            /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)$/,
            "Must be a valid IP address"
        )
        .required("DNS is required"),
    mtu: yup.number().min(1).max(65535).required("MTU is required"),
    bitrateUpValue: yup
        .number()
        .min(1, "Bitrate value must be between 1 and 999")
        .max(999, "Bitrate value must be between 1 and 999")
        .required("Bitrate value is required"),
    bitrateUpUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
    bitrateDownValue: yup
        .number()
        .min(1, "Bitrate value must be between 1 and 999")
        .max(999, "Bitrate value must be between 1 and 999")
        .required("Bitrate value is required"),
    bitrateDownUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
    fiveQi: yup.number().min(0).max(256).required("5QI is required"),
    priorityLevel: yup.number().min(0).max(256).required("Priority Level is required"),
});

const CreateProfileModal: React.FC<CreateProfileModalProps> = ({ open, onClose, onSuccess }) => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

    if (!cookies.user_token) {
        router.push("/login")
    }

    const [formValues, setFormValues] = useState({
        name: "",
        ipPool: "192.168.0.0/24",
        dns: "8.8.8.8",
        mtu: 1500,
        bitrateUpValue: 100,
        bitrateUpUnit: "Mbps",
        bitrateDownValue: 100,
        bitrateDownUnit: "Mbps",
        fiveQi: 1,
        priorityLevel: 1,
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
            const bitrateUplink = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
            const bitrateDownlink = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;
            await createProfile(
                cookies.user_token,
                formValues.name,
                formValues.ipPool,
                formValues.dns,
                formValues.mtu,
                bitrateUplink,
                bitrateDownlink,
                formValues.fiveQi,
                formValues.priorityLevel
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error?.message || "Unknown error occurred.";
            setAlert({
                message: `Failed to create profile: ${errorMessage}`,
            });
            console.error("Failed to create profile:", error);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="create-profile-modal-title"
            aria-describedby="create-profile-modal-description"
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
                <Typography id="create-profile-modal-title" variant="h6" gutterBottom>
                    Create Profile
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
                    onChange={(e) => handleChange("name", e.target.value)}
                    onBlur={() => handleBlur("name")}
                    error={!!errors.name && touched.name}
                    helperText={touched.name ? errors.name : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="IP Pool"
                    value={formValues.ipPool}
                    onChange={(e) => handleChange("ipPool", e.target.value)}
                    onBlur={() => handleBlur("ipPool")}
                    error={!!errors.ipPool && touched.ipPool}
                    helperText={touched.ipPool ? errors.ipPool : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="DNS"
                    value={formValues.dns}
                    onChange={(e) => handleChange("dns", e.target.value)}
                    onBlur={() => handleBlur("dns")}
                    error={!!errors.dns && touched.dns}
                    helperText={touched.dns ? errors.dns : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="MTU"
                    type="number"
                    value={formValues.mtu}
                    onChange={(e) => handleChange("mtu", Number(e.target.value))}
                    onBlur={() => handleBlur("mtu")}
                    error={!!errors.mtu && touched.mtu}
                    helperText={touched.mtu ? errors.mtu : ""}
                    margin="normal"
                />
                {/* Similar changes for other fields */}
                <Box sx={{ textAlign: "right", marginTop: 2 }}>
                    <Button onClick={onClose} sx={{ marginRight: 2 }}>
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
                </Box>
            </Box>
        </Modal>
    );
};

export default CreateProfileModal;
