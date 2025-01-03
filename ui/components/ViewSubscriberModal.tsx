import React, { useState, useEffect } from "react";
import {
    Box,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Typography,
    Button,
    Alert,
    Collapse,
    IconButton,
} from "@mui/material";
import { ContentCopy as CopyIcon } from "@mui/icons-material";
import { getSubscriber } from "@/queries/subscribers";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface ViewSubscriberModalProps {
    open: boolean;
    onClose: () => void;
    imsi: string;
}

const ViewSubscriberModal: React.FC<ViewSubscriberModalProps> = ({
    open,
    onClose,
    imsi,
}) => {
    const router = useRouter();
    const [cookies] = useCookies(["user_token"]);

    if (!cookies.user_token) {
        router.push("/login");
    }

    const [subscriberData, setSubscriberData] = useState({
        imsi: "",
        key: "",
        sequenceNumber: "",
        profileName: "",
    });
    const [loading, setLoading] = useState(false);
    const [alert, setAlert] = useState<{ message: string }>({ message: "" });
    const [keyObfuscated, setKeyObfuscated] = useState(true);

    useEffect(() => {
        const fetchSubscriberData = async () => {
            if (!imsi || !open) return;

            setLoading(true);
            setAlert({ message: "" });

            try {
                const data = await getSubscriber(cookies.user_token, imsi);
                setSubscriberData({
                    imsi: data.imsi,
                    key: data.key,
                    sequenceNumber: data.sequenceNumber,
                    profileName: data.profileName,
                });
            } catch (error: any) {
                setAlert({
                    message: error?.message || "Failed to fetch subscriber data.",
                });
                console.error("Error fetching subscriber data:", error);
            } finally {
                setLoading(false);
            }
        };

        fetchSubscriberData();
    }, [imsi, open, cookies.user_token]);

    const handleCopy = async (value: string, label: string) => {
        try {
            await navigator.clipboard.writeText(value);
            setAlert({ message: `${label} copied to clipboard!` });
        } catch (error) {
            console.error(`Failed to copy ${label}:`, error);
            setAlert({ message: `Failed to copy ${label}.` });
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="view-subscriber-modal-title"
            aria-describedby="view-subscriber-modal-description"
        >
            <DialogTitle>Subscriber Details</DialogTitle>
            <DialogContent dividers>
                <Collapse in={!!alert.message}>
                    <Alert
                        onClose={() => setAlert({ message: "" })}
                        sx={{ mb: 2 }}
                        severity="info"
                    >
                        {alert.message}
                    </Alert>
                </Collapse>
                <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
                    <Typography sx={{ flex: 1 }}>
                        <strong>IMSI:</strong> {subscriberData.imsi}
                    </Typography>
                    <IconButton
                        onClick={() => handleCopy(subscriberData.imsi, "IMSI")}
                        aria-label="Copy IMSI"
                    >
                        <CopyIcon />
                    </IconButton>
                </Box>
                <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
                    <Typography sx={{ flex: 1 }}>
                        <strong>Key:</strong>{" "}
                        {keyObfuscated ? "••••••••••••••••••••••••••••••••" : subscriberData.key}
                    </Typography>
                    <Button
                        variant="text"
                        onClick={() => setKeyObfuscated(!keyObfuscated)}
                    >
                        {keyObfuscated ? "Show" : "Hide"}
                    </Button>
                    <IconButton
                        onClick={() => handleCopy(subscriberData.key, "Key")}
                        aria-label="Copy Key"
                    >
                        <CopyIcon />
                    </IconButton>
                </Box>
                <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
                    <Typography sx={{ flex: 1 }}>
                        <strong>Sequence Number:</strong> {subscriberData.sequenceNumber}
                    </Typography>
                    <IconButton
                        onClick={() =>
                            handleCopy(subscriberData.sequenceNumber, "Sequence Number")
                        }
                        aria-label="Copy Sequence Number"
                    >
                        <CopyIcon />
                    </IconButton>
                </Box>
                <Box sx={{ mb: 2 }}>
                    <Typography>
                        <strong>Profile Name:</strong> {subscriberData.profileName}
                    </Typography>
                </Box>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} sx={{ marginRight: 2 }}>
                    Close
                </Button>
            </DialogActions>
        </Dialog >
    );
};

export default ViewSubscriberModal;
