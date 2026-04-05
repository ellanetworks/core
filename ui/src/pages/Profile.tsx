import React, { useEffect, useState } from "react";
import { Box, Typography, Skeleton } from "@mui/material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import EditMyUserPasswordModal from "@/components/EditMyUserPasswordModal";
import { getLoggedInUser, type APIUser } from "@/queries/users";
import {
  listAPITokens,
  deleteAPIToken,
  type APIToken,
  type ListAPITokensResponse,
} from "@/queries/api_tokens";
import { useAuth } from "@/contexts/AuthContext";
import { useNavigate } from "react-router-dom";
import { useSnackbar } from "@/contexts/SnackbarContext";
import UserAccountCard from "@/components/UserAccountCard";
import UserPasswordCard from "@/components/UserPasswordCard";
import UserAPITokensCard from "@/components/UserAPITokensCard";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

export default function Profile() {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  const queryClient = useQueryClient();

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);

  const { showSnackbar } = useSnackbar();

  const { data: loggedInUser, isLoading: loadingUser } = useQuery<APIUser>({
    queryKey: ["loggedInUser"],
    queryFn: () => getLoggedInUser(accessToken!),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
  });

  const { data: tokensData, isLoading: loadingTokens } =
    useQuery<ListAPITokensResponse>({
      queryKey: ["myAPITokens"],
      queryFn: () => listAPITokens(accessToken!, 1, 100),
      enabled: authReady && !!accessToken,
      refetchInterval: 5000,
    });

  const apiTokens: APIToken[] = tokensData?.items ?? [];

  const handlePasswordSuccess = () => {
    setEditPasswordModalOpen(false);
    showSnackbar("Password updated successfully.", "success");
  };

  const handleDeleteToken = async (token: APIToken) => {
    if (!accessToken) return;
    try {
      await deleteAPIToken(accessToken, token.id);
      showSnackbar(
        `API token "${token.name}" deleted successfully.`,
        "success",
      );
      queryClient.invalidateQueries({ queryKey: ["myAPITokens"] });
    } catch (error) {
      showSnackbar(
        `Failed to delete token: ${
          error instanceof Error ? error.message : "Unknown error"
        }`,
        "error",
      );
    }
  };

  const handleTokenCreated = () => {
    queryClient.invalidateQueries({ queryKey: ["myAPITokens"] });
  };

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Typography variant="h4" sx={{ mb: 1 }}>
        My Profile
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mb: 3 }}>
        Manage how you authenticate with Ella Core.
      </Typography>

      {/* Two-column body */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
          alignItems: "stretch",
        }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          {loadingUser || !loggedInUser ? (
            <Skeleton variant="rounded" height={140} />
          ) : (
            <UserAccountCard user={loggedInUser} />
          )}
        </Box>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <UserPasswordCard
            onChangePassword={() => setEditPasswordModalOpen(true)}
            disabled={loadingUser || !loggedInUser}
          />
        </Box>
      </Box>

      {/* API Tokens — full width */}
      <Box sx={{ mt: 3 }}>
        {loadingTokens && apiTokens.length === 0 ? (
          <Skeleton variant="rounded" height={300} />
        ) : (
          <UserAPITokensCard
            tokens={apiTokens}
            onDeleteToken={handleDeleteToken}
            onTokenCreated={handleTokenCreated}
          />
        )}
      </Box>

      {isEditPasswordModalOpen && (
        <EditMyUserPasswordModal
          open
          onClose={() => setEditPasswordModalOpen(false)}
          onSuccess={handlePasswordSuccess}
        />
      )}
    </Box>
  );
}
