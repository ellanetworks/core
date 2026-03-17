import React, { useEffect, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Skeleton,
  Typography,
} from "@mui/material";
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
import SubscriberProtocolChart from "@/components/SubscriberProtocolChart";
import { MAX_WIDTH } from "@/utils/layout";

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
    if (!imsi || !accessToken) return;
    try {
      await deleteSubscriber(accessToken, imsi);
      setDeleteConfirmOpen(false);
      showSnackbar(`Subscriber "${imsi}" deleted successfully.`, "success");
      navigate("/subscribers");
    } catch (err) {
      setDeleteConfirmOpen(false);
      showSnackbar(
        `Failed to delete subscriber: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  if (!authReady || isLoading) {
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
          {/* Header skeleton */}
          <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />

          {/* Two-column skeleton */}
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
              gap: 3,
            }}
          >
            <Skeleton variant="rounded" height={320} />
            <Skeleton variant="rounded" height={320} />
          </Box>

          {/* Traffic card skeleton */}
          <Skeleton variant="rounded" height={340} sx={{ mt: 3 }} />
        </Box>
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
                <Typography component="span" variant="h4">
                  {subscriber.imsi}
                </Typography>
              </Typography>
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
            gridTemplateRows: { xs: "auto auto auto", md: "auto auto" },
            gap: 3,
            alignItems: "stretch",
          }}
        >
          {/* Left column */}
          <Box
            sx={{
              gridColumn: 1,
              gridRow: { xs: 1, md: "1 / span 2" },
              display: "flex",
              flexDirection: "column",
              gap: 3,
            }}
          >
            <SubscriberProvisioningCard
              subscriber={subscriber}
              onEditPolicy={canEdit ? () => setEditModalOpen(true) : undefined}
            />
          </Box>

          {/* Right column */}
          <Box
            sx={{
              gridColumn: { xs: 1, md: 2 },
              gridRow: { xs: 2, md: "1 / span 2" },
              display: "flex",
              flexDirection: "column",
              gap: 3,
            }}
          >
            <SubscriberConnectionCard
              status={subscriber.status}
              sessions={subscriber.pdu_sessions}
              loading={isLoading}
              ipAddress={
                subscriber.pdu_sessions && subscriber.pdu_sessions.length > 0
                  ? subscriber.pdu_sessions[0].ipAddress
                  : subscriber.status.ipAddress
              }
            />
          </Box>
        </Box>

        {/* Traffic card */}
        <Card variant="outlined" sx={{ mt: 3 }}>
          <CardContent>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 2,
              }}
            >
              <Typography variant="h6">Traffic Summary</Typography>
              <Button
                component={RouterLink}
                to={`/traffic/usage?subscriber_id=${subscriber.imsi}`}
                size="small"
                sx={{
                  color: (theme) => theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                View traffic for {subscriber.imsi} →
              </Button>
            </Box>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                gap: 3,
                alignItems: "start",
              }}
            >
              <SubscriberUsageChart imsi={subscriber.imsi} embedded />
              <SubscriberProtocolChart imsi={subscriber.imsi} />
            </Box>
          </CardContent>
        </Card>
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
