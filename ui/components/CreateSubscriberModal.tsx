import React, { useState, useEffect } from "react";
import {
    Box,
    Modal,
    TextField,
    Button,
    Typography,
    Alert,
    Collapse,
    MenuItem,
    Select,
    InputLabel,
    FormControl,
    FormGroup,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createSubscriber } from "@/queries/subscribers";
import { listProfiles } from "@/queries/profiles";
import { getNetwork } from "@/queries/network";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface CreateSubscriberModalProps {
    open: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

const schema = yup.object().shape({
    msin: yup
        .string()
        .length(10, "MSIN must be exactly 10 digits long.")
        .matches(/^\d+$/, "MSIN must be numeric.")
        .required("MSIN is required."),
    opc: yup
        .string()
        .matches(/^[0-9a-fA-F]{32}$/, "OPC must be a 32-character hexadecimal string.")
        .required("OPC is required."),
    key: yup
        .string()
        .matches(/^[0-9a-fA-F]{32}$/, "Key must be a 32-character hexadecimal string.")
        .required("Key is required."),
    sequenceNumber: yup
        .string()
        .matches(/^[0-9a-fA-F]{12}$/, "Sequence Number must be a 6-byte (12-character) hexadecimal string.")
        .required("Sequence Number is required."),
    profileName: yup
        .string()
        .required("Profile Name is required."),
});

const CreateSubscriberModal: React.FC<CreateSubscriberModalProps> = ({ open, onClose, onSuccess }) => {
    const router = useRouter();
    const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

    if (!cookies.user_token) {
        router.push("/login")
    }
    const [formValues, setFormValues] = useState({
        msin: "",
        opc: "",
        key: "",
        sequenceNumber: "",
        profileName: "",
    });

    const [mcc, setMcc] = useState("");
    const [mnc, setMnc] = useState("");
    const [profiles, setProfiles] = useState<string[]>([]);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [touched, setTouched] = useState<Record<string, boolean>>({});
    const [isValid, setIsValid] = useState(false);
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });

    useEffect(() => {
        const fetchNetworkAndProfiles = async () => {
            try {
                const network = await getNetwork(cookies.user_token);
                setMcc(network.mcc);
                setMnc(network.mnc);

                const profileData = await listProfiles(cookies.user_token);
                setProfiles(profileData.map((profile: any) => profile.name));
            } catch (error) {
                console.error("Failed to fetch data:", error);
            }
        };

        if (open) {
            fetchNetworkAndProfiles();
        }
    }, [open]);

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
            const imsi = `${mcc}${mnc}${formValues.msin}`;
            await createSubscriber(
                cookies.user_token,
                imsi,
                formValues.opc,
                formValues.key,
                formValues.sequenceNumber,
                formValues.profileName
            );
            onClose();
            onSuccess();
        } catch (error: any) {
            const errorMessage = error.message || "Unknown error occurred.";
            setAlert({
                message: `Failed to create subscriber: ${errorMessage}`,
            });
            console.error("Failed to create subscriber:", error);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-labelledby="create-subscriber-modal-title"
            aria-describedby="create-subscriber-modal-description"
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
                <Typography id="create-subscriber-modal-title" variant="h6" gutterBottom>
                    Create Subscriber
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
                <FormGroup sx={{ mb: 2, p: 2, border: "1px solid #ccc", borderRadius: 1 }}>
                    <Typography
                        variant="subtitle1"
                        gutterBottom
                    >
                        IMSI
                    </Typography>
                    <Box display="flex" gap={2}>
                        <TextField
                            label="MCC"
                            value={mcc}
                            disabled
                            margin="normal"
                            sx={{ flex: 1 }}
                        />
                        <TextField
                            label="MNC"
                            value={mnc}
                            disabled
                            margin="normal"
                            sx={{ flex: 1 }}
                        />
                        <TextField
                            label="MSIN"
                            value={formValues.msin}
                            onChange={(e) => handleChange("msin", e.target.value)}
                            onBlur={() => handleBlur("msin")}
                            error={!!errors.msin && touched.msin}
                            helperText={touched.msin ? errors.msin : ""}
                            margin="normal"
                            sx={{ flex: 2 }}
                        />
                    </Box>
                </FormGroup>
                <TextField
                    fullWidth
                    label="OPC"
                    value={formValues.opc}
                    onChange={(e) => handleChange("opc", e.target.value)}
                    onBlur={() => handleBlur("opc")}
                    error={!!errors.opc && touched.opc}
                    helperText={touched.opc ? errors.opc : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Key"
                    value={formValues.key}
                    onChange={(e) => handleChange("key", e.target.value)}
                    onBlur={() => handleBlur("key")}
                    error={!!errors.key && touched.key}
                    helperText={touched.key ? errors.key : ""}
                    margin="normal"
                />
                <TextField
                    fullWidth
                    label="Sequence Number"
                    value={formValues.sequenceNumber}
                    onChange={(e) => handleChange("sequenceNumber", e.target.value)}
                    onBlur={() => handleBlur("sequenceNumber")}
                    error={!!errors.sequenceNumber && touched.sequenceNumber}
                    helperText={touched.sequenceNumber ? errors.sequenceNumber : ""}
                    margin="normal"
                />
                <FormControl fullWidth margin="normal">
                    <InputLabel id="profile-name-label">Profile Name</InputLabel>
                    <Select
                        labelId="profile-name-label"
                        value={formValues.profileName}
                        onChange={(e) => handleChange("profileName", e.target.value)}
                        onBlur={() => handleBlur("profileName")}
                        error={!!errors.profileName && touched.profileName}
                    >
                        {profiles.map((profile) => (
                            <MenuItem key={profile} value={profile}>
                                {profile}
                            </MenuItem>
                        ))}
                    </Select>
                    {touched.profileName && errors.profileName && (
                        <Typography color="error" variant="caption">
                            {errors.profileName}
                        </Typography>
                    )}
                </FormControl>
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

export default CreateSubscriberModal;
