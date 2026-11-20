// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import { Alert, Box, Typography, Skeleton } from "@mui/material";
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
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { useSnackbar } from "@/contexts/SnackbarContext";
import UserAccountCard from "@/components/UserAccountCard";
import UserPasswordCard from "@/components/UserPasswordCard";
import UserAPITokensCard from "@/components/UserAPITokensCard";
import QueryState from "@/components/QueryState";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import PageTitle from "@/components/PageTitle";
import { PRODUCT } from "@/utils/product";
import {
  forgetLinking,
  getOIDCLogin,
  isLinking,
  OIDC_LINK_PATH,
  oidcLinkError,
  rememberLinking,
  type OIDCLogin,
} from "@/queries/oidc";

type LinkResult = { error: string } | { linked: true } | null;

export default function Profile() {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  const queryClient = useQueryClient();

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const location = useLocation();
  const [params, setParams] = useSearchParams();
  const [linkResult, setLinkResult] = useState<LinkResult>(() => {
    const error = params.get("error");
    if (error) return { error };
    return isLinking() ? { linked: true } : null;
  });

  useEffect(() => {
    forgetLinking();
    if (params.has("error")) {
      setParams(
        (current) => {
          const next = new URLSearchParams(current);
          next.delete("error");
          return next;
        },
        { replace: true, state: location.state },
      );
    }
  }, [params, setParams, location.state]);

  const oidcLoginQuery = useQuery<OIDCLogin>({
    queryKey: ["oidcLogin"],
    queryFn: getOIDCLogin,
    retry: false,
  });
  const ssoName = oidcLoginQuery.data?.enabled
    ? oidcLoginQuery.data.display_name
    : undefined;

  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);

  const { showSnackbar } = useSnackbar();

  const userQuery = useQuery<APIUser>({
    queryKey: ["loggedInUser"],
    queryFn: () => getLoggedInUser(accessToken!),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    retry: false,
  });

  const tokensQuery = useQuery<ListAPITokensResponse>({
    queryKey: ["myAPITokens"],
    queryFn: () => listAPITokens(accessToken!, 1, 100),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    retry: false,
  });

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
      <PageTitle title="My Profile" sx={{ mb: 1 }} />
      <Typography variant="body1" color="textSecondary" sx={{ mb: 3 }}>
        Manage how you authenticate with {PRODUCT.name}.
      </Typography>

      {linkResult && "error" in linkResult && (
        <Alert
          severity="error"
          onClose={() => setLinkResult(null)}
          sx={{ mb: 3 }}
        >
          {oidcLinkError(linkResult.error)}
        </Alert>
      )}
      {linkResult && "linked" in linkResult && userQuery.data?.oidc_linked && (
        <Alert
          severity="success"
          onClose={() => setLinkResult(null)}
          sx={{ mb: 3 }}
        >
          Your account is linked. You can now log in with{" "}
          {ssoName ?? "single sign-on"}.
        </Alert>
      )}

      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
          alignItems: "stretch",
        }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <QueryState
            query={userQuery}
            resource="your account"
            loading={<Skeleton variant="rounded" height={140} />}
          >
            {(loggedInUser) => (
              <UserAccountCard
                user={loggedInUser}
                link={
                  ssoName
                    ? {
                        label: `Link ${ssoName} account`,
                        href: OIDC_LINK_PATH,
                        onClick: rememberLinking,
                      }
                    : undefined
                }
              />
            )}
          </QueryState>
        </Box>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          {userQuery.data?.password_set === false ? (
            <UserPasswordCard description="You sign in with single sign-on, and have no password." />
          ) : (
            <UserPasswordCard
              description="Changing your password ends your other sessions."
              onChangePassword={() => setEditPasswordModalOpen(true)}
              disabled={userQuery.data === undefined}
            />
          )}
        </Box>
      </Box>

      <Box sx={{ mt: 3 }}>
        <QueryState
          query={tokensQuery}
          resource="your API tokens"
          loading={<Skeleton variant="rounded" height={300} />}
        >
          {(data) => (
            <UserAPITokensCard
              tokens={data.items ?? []}
              onDeleteToken={handleDeleteToken}
              onTokenCreated={handleTokenCreated}
            />
          )}
        </QueryState>
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
