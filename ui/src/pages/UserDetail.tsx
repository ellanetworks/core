// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import { Box, Button, Skeleton } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
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
  type ListAuditLogsResponse,
} from "@/queries/audit_logs";
import { deleteUser, removeUserPassword, unlinkUser } from "@/queries/users";
import {
  getOIDC,
  OIDC_QUERY_KEY,
  roleFromGroups,
  type OIDCSettings,
} from "@/queries/oidc";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditUserModal from "@/components/EditUserModal";
import EditUserPasswordModal from "@/components/EditUserPasswordModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import ConfirmDialog from "@/components/form/ConfirmDialog";
import UserAccountCard from "@/components/UserAccountCard";
import UserPasswordCard from "@/components/UserPasswordCard";
import UserAPITokensCard from "@/components/UserAPITokensCard";
import UserAuditLogsCard from "@/components/UserAuditLogsCard";
import QueryState from "@/components/QueryState";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import PageTitle from "@/components/PageTitle";

const removePasswordDescription = (
  email: string,
  ssoEnabled: boolean,
  linked: boolean,
) => {
  const consequence = !ssoEnabled
    ? "Single sign-on is off, so they cannot sign in until it is on."
    : linked
      ? "They can then sign in only with single sign-on."
      : "They have not signed in with single sign-on yet.";
  return `Remove the password of ${email}? ${consequence} Their sessions end.`;
};

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
  const [confirming, setConfirming] = useState<
    "removePassword" | "unlink" | null
  >(null);

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const userQuery = useQuery<APIUser>({
    queryKey: ["user", email],
    queryFn: () => getUser(accessToken!, email!),
    enabled: authReady && !!accessToken && !!email,
    refetchInterval: 5000,
    retry: false,
  });

  const oidcQuery = useQuery<OIDCSettings>({
    queryKey: OIDC_QUERY_KEY,
    queryFn: () => getOIDC(accessToken!),
    enabled: authReady && !!accessToken && isAdmin,
    retry: false,
  });

  const tokensQuery = useQuery<ListAPITokensResponse>({
    queryKey: ["userAPITokens", email],
    queryFn: () => listUserAPITokens(accessToken!, email!, 1, 12),
    enabled: authReady && !!accessToken && !!email && isAdmin,
    refetchInterval: 5000,
    retry: false,
  });

  const auditQuery = useQuery<ListAuditLogsResponse>({
    queryKey: ["userAuditLogs", email],
    queryFn: () => listAuditLogs(accessToken!, 1, 10, { user: email! }),
    enabled: authReady && !!accessToken && !!email,
    refetchInterval: 5000,
    retry: false,
  });

  const handleDeleteConfirm = async () => {
    if (!email || !accessToken) return;
    try {
      await deleteUser(accessToken, email);
      setDeleteConfirmOpen(false);
      showSnackbar(`User "${email}" deleted successfully.`, "success");
      navigate("/users");
    } catch (err) {
      setDeleteConfirmOpen(false);
      showSnackbar(
        `Failed to delete user: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const handleUserChange = async (
    request: () => Promise<void>,
    done: string,
    failed: string,
  ) => {
    try {
      await request();
      setConfirming(null);
      void queryClient.invalidateQueries({ queryKey: ["users"] });
      void queryClient.invalidateQueries({ queryKey: ["user", email] });
      showSnackbar(done, "success");
    } catch (err) {
      setConfirming(null);
      showSnackbar(
        `${failed}: ${err instanceof Error ? err.message : "Unknown error"}`,
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

  const loadingBody = (
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
  );

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
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
          <PageTitle
            parent={{ label: "Users", to: "/users" }}
            title={email ?? ""}
          />
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

      <QueryState query={userQuery} resource="this user" loading={loadingBody}>
        {(user) => (
          <>
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
                  fromGroups={roleFromGroups(user, oidcQuery.data)}
                  onUnlink={
                    isAdmin && !isSelf
                      ? () => setConfirming("unlink")
                      : undefined
                  }
                />
              </Box>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
                <UserPasswordCard
                  description={
                    user.password_set
                      ? undefined
                      : "This user has no password and signs in only with single sign-on."
                  }
                  changeLabel={
                    user.password_set ? "Change Password" : "Set Password"
                  }
                  onChangePassword={() => setEditPasswordModalOpen(true)}
                  onRemovePassword={
                    isAdmin && !isSelf && user.password_set
                      ? () => setConfirming("removePassword")
                      : undefined
                  }
                  disabled={!isAdmin}
                />
              </Box>
            </Box>

            {isAdmin && (
              <Box sx={{ mt: 3 }}>
                <QueryState
                  query={tokensQuery}
                  resource="this user's API tokens"
                  loading={<Skeleton variant="rounded" height={300} />}
                >
                  {(data) => (
                    <UserAPITokensCard
                      tokens={data.items ?? []}
                      onDeleteToken={handleDeleteToken}
                      onTokenCreated={handleTokenCreated}
                      targetEmail={email}
                    />
                  )}
                </QueryState>
              </Box>
            )}

            <Box sx={{ mt: 3 }}>
              <QueryState
                query={auditQuery}
                resource="this user's audit logs"
                loading={<Skeleton variant="rounded" height={300} />}
              >
                {(data) => (
                  <UserAuditLogsCard logs={data.items ?? []} email={email!} />
                )}
              </QueryState>
            </Box>

            {isEditModalOpen && (
              <EditUserModal
                open
                onClose={() => setEditModalOpen(false)}
                onSuccess={() => {
                  userQuery.refetch();
                  showSnackbar("User updated successfully.", "success");
                }}
                initialData={user}
              />
            )}
            {isEditPasswordModalOpen && (
              <EditUserPasswordModal
                open
                onClose={() => setEditPasswordModalOpen(false)}
                onSuccess={() => {
                  void queryClient.invalidateQueries({ queryKey: ["users"] });
                  void userQuery.refetch();
                  showSnackbar("Password updated successfully.", "success");
                }}
                initialData={{ email: user.email }}
              />
            )}
          </>
        )}
      </QueryState>

      <ConfirmDialog
        open={confirming === "removePassword"}
        onClose={() => setConfirming(null)}
        onConfirm={() =>
          handleUserChange(
            () => removeUserPassword(accessToken!, email!),
            "Password removed successfully.",
            "Failed to remove the password",
          )
        }
        title="Remove Password"
        description={removePasswordDescription(
          email ?? "",
          oidcQuery.data?.enabled === true,
          userQuery.data?.oidc_linked === true,
        )}
        confirmLabel="Remove"
        confirmingLabel="Removing…"
      />
      <ConfirmDialog
        open={confirming === "unlink"}
        onClose={() => setConfirming(null)}
        onConfirm={() =>
          handleUserChange(
            () => unlinkUser(accessToken!, email!),
            "User unlinked successfully.",
            "Failed to unlink the user",
          )
        }
        title="Unlink from Identity Provider"
        description={`Unlink ${email} from their identity provider account? Their single sign-on sessions end, and their next sign-in links them again.`}
        confirmLabel="Unlink"
        confirmingLabel="Unlinking…"
      />

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
