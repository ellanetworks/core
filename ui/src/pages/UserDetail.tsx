import React, { useEffect, useState } from "react";
import { Box, Button, Skeleton, Typography } from "@mui/material";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getUser, type APIUser } from "@/queries/users";
import {
  listUserAPITokens,
  deleteUserAPIToken,
  type APIToken,
  type ListAPITokensResponse,
} from "@/queries/api_tokens";
import {
  listAuditLogs,
  type APIAuditLog,
  type ListAuditLogsResponse,
} from "@/queries/audit_logs";
import { deleteUser } from "@/queries/users";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditUserModal from "@/components/EditUserModal";
import EditUserPasswordModal from "@/components/EditUserPasswordModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import UserAccountCard from "@/components/UserAccountCard";
import UserPasswordCard from "@/components/UserPasswordCard";
import UserAPITokensCard from "@/components/UserAPITokensCard";
import UserAuditLogsCard from "@/components/UserAuditLogsCard";

const MAX_WIDTH = 1400;

const UserDetail: React.FC = () => {
  const { email } = useParams<{ email: string }>();
  const navigate = useNavigate();
  const { role, email: loggedInEmail, accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const isAdmin = role === "Admin";
  const isSelf = loggedInEmail === email;

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: user,
    isLoading,
    error,
    refetch: refetchUser,
  } = useQuery<APIUser>({
    queryKey: ["user", email],
    queryFn: () => getUser(accessToken!, email!),
    enabled: authReady && !!accessToken && !!email,
    refetchInterval: 5000,
  });

  const { data: tokensData } = useQuery<ListAPITokensResponse>({
    queryKey: ["userAPITokens", email],
    queryFn: () => listUserAPITokens(accessToken!, email!, 1, 12),
    enabled: authReady && !!accessToken && !!email && isAdmin,
    refetchInterval: 5000,
  });

  const { data: auditData } = useQuery<ListAuditLogsResponse>({
    queryKey: ["userAuditLogs", email],
    queryFn: () => listAuditLogs(accessToken!, 1, 10, email!),
    enabled: authReady && !!accessToken && !!email,
    refetchInterval: 5000,
  });

  const handleDeleteConfirm = async () => {
    setDeleteConfirmOpen(false);
    if (!email || !accessToken) return;
    try {
      await deleteUser(accessToken, email);
      showSnackbar(`User "${email}" deleted successfully.`, "success");
      navigate("/users");
    } catch (err) {
      showSnackbar(
        `Failed to delete user: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const handleDeleteToken = async (token: APIToken) => {
    if (!email || !accessToken) return;
    try {
      await deleteUserAPIToken(accessToken, email, token.id);
      showSnackbar(
        `API token "${token.name}" deleted successfully.`,
        "success",
      );
      queryClient.invalidateQueries({ queryKey: ["userAPITokens", email] });
    } catch (err) {
      showSnackbar(
        `Failed to delete token: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const handleTokenCreated = () => {
    queryClient.invalidateQueries({ queryKey: ["userAPITokens", email] });
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
          <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
              gap: 3,
            }}
          >
            <Skeleton variant="rounded" height={180} />
            <Skeleton variant="rounded" height={180} />
          </Box>
          <Skeleton variant="rounded" height={300} sx={{ mt: 3 }} />
          <Skeleton variant="rounded" height={300} sx={{ mt: 3 }} />
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
          {error instanceof Error ? error.message : "Failed to load user."}
        </Typography>
        <Button variant="outlined" component={RouterLink} to="/users">
          Back to Users
        </Button>
      </Box>
    );
  }

  if (!user) return null;

  const tokens: APIToken[] = tokensData?.items ?? [];
  const auditLogs: APIAuditLog[] = auditData?.items ?? [];

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
        {/* Header / Breadcrumb */}
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
            <Typography
              variant="h4"
              sx={{ display: "flex", alignItems: "baseline" }}
            >
              <Typography
                component={RouterLink}
                to="/users"
                variant="h4"
                sx={{
                  color: "text.secondary",
                  textDecoration: "none",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                Users
              </Typography>
              <Typography
                component="span"
                variant="h4"
                sx={{ color: "text.secondary", mx: 1 }}
              >
                /
              </Typography>
              <Typography component="span" variant="h4">
                {user.email}
              </Typography>
            </Typography>
          </Box>
          {isAdmin && !isSelf && (
            <Button
              variant="outlined"
              color="error"
              onClick={() => setDeleteConfirmOpen(true)}
            >
              Delete
            </Button>
          )}
        </Box>

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
            <UserAccountCard
              user={user}
              canEdit={isAdmin && !isSelf}
              onEdit={() => setEditModalOpen(true)}
            />
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <UserPasswordCard
              onChangePassword={() => setEditPasswordModalOpen(true)}
              disabled={!isAdmin}
            />
          </Box>
        </Box>

        {/* API Tokens — full width */}
        {isAdmin && (
          <Box sx={{ mt: 3 }}>
            <UserAPITokensCard
              tokens={tokens}
              onDeleteToken={handleDeleteToken}
              onTokenCreated={handleTokenCreated}
              targetEmail={email}
            />
          </Box>
        )}

        {/* Recent Audit Logs — full width */}
        <Box sx={{ mt: 3 }}>
          <UserAuditLogsCard logs={auditLogs} />
        </Box>
      </Box>

      {/* Modals */}
      {isEditModalOpen && (
        <EditUserModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={() => {
            refetchUser();
            showSnackbar("User updated successfully.", "success");
          }}
          initialData={{ email: user.email, role_id: user.role_id }}
        />
      )}
      {isEditPasswordModalOpen && (
        <EditUserPasswordModal
          open
          onClose={() => setEditPasswordModalOpen(false)}
          onSuccess={() => {
            showSnackbar("Password updated successfully.", "success");
          }}
          initialData={{ email: user.email }}
        />
      )}
      {isDeleteConfirmOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteConfirmOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the user "${email}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default UserDetail;
