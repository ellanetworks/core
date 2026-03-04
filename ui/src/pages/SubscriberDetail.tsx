import React, { useEffect, useState } from "react";
import { Box, Button, Chip, CircularProgress, Typography } from "@mui/material";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  getSubscriber,
  deleteSubscriber,
  type APISubscriber,
} from "@/queries/subscribers";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditSubscriberModal from "@/components/EditSubscriberModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import SubscriberProvisioningCard from "@/components/SubscriberProvisioningCard";
import SubscriberConnectionCard from "@/components/SubscriberConnectionCard";
import SubscriberUsageChart from "@/components/SubscriberUsageChart";
import SubscriberRecentFlows from "@/components/SubscriberRecentFlows";

const MAX_WIDTH = 1400;

const SubscriberDetail: React.FC = () => {
  const { imsi } = useParams<{ imsi: string }>();
  const navigate = useNavigate();
  const { role, accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: subscriber,
    isLoading,
    error,
    refetch,
  } = useQuery<APISubscriber>({
    queryKey: ["subscriber", imsi],
    queryFn: () => getSubscriber(accessToken!, imsi!),
    enabled: authReady && !!accessToken && !!imsi,
    refetchInterval: 5000,
  });

  const handleDeleteConfirm = async () => {
    setDeleteConfirmOpen(false);
    if (!imsi || !accessToken) return;
    try {
      await deleteSubscriber(accessToken, imsi);
      showSnackbar(`Subscriber "${imsi}" deleted successfully.`, "success");
      navigate("/subscribers");
    } catch (err) {
      showSnackbar(
        `Failed to delete subscriber: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  if (!authReady || isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          mt: 6,
          gap: 2,
        }}
      >
        <Typography color="error">
          {error instanceof Error
            ? error.message
            : "Failed to load subscriber."}
        </Typography>
        <Button variant="outlined" component={RouterLink} to="/subscribers">
          Back to Subscribers
        </Button>
      </Box>
    );
  }

  if (!subscriber) return null;

  const registered = Boolean(subscriber.status?.registered);

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        {/* Header */}
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            alignItems: { xs: "flex-start", sm: "center" },
            gap: 2,
            mb: 3,
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Box
              sx={{ display: "flex", alignItems: "center", gap: 2, mb: 0.5 }}
            >
              <Typography
                variant="h4"
                sx={{ display: "flex", alignItems: "baseline", gap: 0 }}
              >
                <Typography
                  component={RouterLink}
                  to="/subscribers"
                  variant="h4"
                  sx={{
                    color: "text.secondary",
                    textDecoration: "none",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  Subscribers
                </Typography>
                <Typography
                  component="span"
                  variant="h4"
                  sx={{ color: "text.secondary", mx: 1 }}
                >
                  /
                </Typography>
                <Typography
                  component="span"
                  variant="h4"
                  sx={{ fontFamily: "monospace" }}
                >
                  {subscriber.imsi}
                </Typography>
              </Typography>
              <Chip
                size="small"
                label={registered ? "Registered" : "Deregistered"}
                color={registered ? "success" : "default"}
                variant="filled"
              />
            </Box>
          </Box>
          {canEdit && (
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button
                variant="outlined"
                color="error"
                onClick={() => setDeleteConfirmOpen(true)}
              >
                Delete
              </Button>
            </Box>
          )}
        </Box>

        {/* Two-column body */}
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
            alignItems: "start",
          }}
        >
          {/* Left column */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <SubscriberProvisioningCard
              subscriber={subscriber}
              onEditPolicy={canEdit ? () => setEditModalOpen(true) : undefined}
            />
            <SubscriberUsageChart imsi={subscriber.imsi} />
            <SubscriberRecentFlows imsi={subscriber.imsi} />
          </Box>

          {/* Right column */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <SubscriberConnectionCard status={subscriber.status} />
          </Box>
        </Box>
      </Box>

      {/* Modals */}
      {isEditModalOpen && (
        <EditSubscriberModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Subscriber updated successfully.", "success");
          }}
          initialData={{
            imsi: subscriber.imsi,
            policyName: subscriber.policyName,
          }}
        />
      )}
      {isDeleteConfirmOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteConfirmOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the subscriber "${imsi}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default SubscriberDetail;
