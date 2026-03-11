import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Typography,
  Skeleton,
  Select,
  MenuItem,
  Button,
  Stack,
} from "@mui/material";
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
import { MAX_WIDTH } from "@/utils/layout";

export default function Profile() {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);

  const { showSnackbar } = useSnackbar();

  const [loggedInUser, setLoggedInUser] = useState<APIUser | null>(null);
  const [loadingUser, setLoadingUser] = useState(true);

  // --- API tokens state (paginated) ---
  const [apiTokens, setAPITokens] = useState<APIToken[]>([]);
  const [loadingTokens, setLoadingTokens] = useState(true);
  const [initialTokenLoadDone, setInitialTokenLoadDone] = useState(false);
  const [page, setPage] = useState<number>(1);
  const [perPage, setPerPage] = useState<number>(10);
  const [totalCount, setTotalCount] = useState<number>(0);

  const fetchUser = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      setLoadingUser(true);
      const data = await getLoggedInUser(accessToken);
      setLoggedInUser(data);
    } catch (error) {
      console.error("Error fetching user:", error);
      showSnackbar("Failed to load profile info.", "error");
    } finally {
      setLoadingUser(false);
    }
  }, [accessToken, authReady]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  const handlePasswordSuccess = () => {
    setEditPasswordModalOpen(false);
    showSnackbar("Password updated successfully.", "success");
  };

  const fetchAPITokens = useCallback(
    async (p = page, pp = perPage) => {
      if (!authReady || !accessToken) return;
      setLoadingTokens(true);
      try {
        const res: ListAPITokensResponse = await listAPITokens(
          accessToken,
          p,
          pp,
        );
        setAPITokens(res.items ?? []);
        setTotalCount(res.total_count ?? 0);
        setInitialTokenLoadDone(true);
      } catch (error) {
        console.error("Error fetching API Tokens:", error);
        showSnackbar("Failed to load API tokens.", "error");
        setAPITokens([]);
        setTotalCount(0);
      } finally {
        setLoadingTokens(false);
      }
    },
    [accessToken, authReady, page, perPage],
  );

  useEffect(() => {
    fetchAPITokens(page, perPage);
  }, [fetchAPITokens, page, perPage]);

  const handleDeleteToken = async (token: APIToken) => {
    if (!accessToken) return;
    try {
      await deleteAPIToken(accessToken, token.id);
      showSnackbar(
        `API token "${token.name}" deleted successfully.`,
        "success",
      );
      const remainingOnPage = apiTokens.length - 1;
      if (remainingOnPage <= 0 && page > 1) {
        const newPage = page - 1;
        setPage(newPage);
        fetchAPITokens(newPage, perPage);
      } else {
        fetchAPITokens(page, perPage);
      }
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
    setPage(1);
    fetchAPITokens(1, perPage);
  };

  const totalPages = Math.max(1, Math.ceil(totalCount / perPage));
  const from = totalCount === 0 ? 0 : (page - 1) * perPage + 1;
  const to = Math.min(page * perPage, totalCount);

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
          {!initialTokenLoadDone ? (
            <Skeleton variant="rounded" height={300} />
          ) : (
            <>
              <UserAPITokensCard
                tokens={apiTokens}
                onDeleteToken={handleDeleteToken}
                onTokenCreated={handleTokenCreated}
              />

              {/* Pagination */}
              <Stack
                direction={{ xs: "column", sm: "row" }}
                spacing={2}
                alignItems={{ xs: "stretch", sm: "center" }}
                justifyContent="space-between"
                sx={{ mt: 2 }}
              >
                <Typography variant="body2" color="text.secondary">
                  {totalCount > 0
                    ? `${from}–${to} of ${totalCount}`
                    : "0 items"}
                </Typography>

                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="body2" color="text.secondary">
                    Rows per page
                  </Typography>
                  <Select
                    size="small"
                    value={perPage}
                    onChange={(e) => {
                      const next = Number(e.target.value);
                      setPerPage(next);
                      setPage(1);
                    }}
                  >
                    {[5, 10, 25, 50].map((n) => (
                      <MenuItem key={n} value={n}>
                        {n}
                      </MenuItem>
                    ))}
                  </Select>

                  <Button
                    size="small"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={page <= 1 || loadingTokens}
                  >
                    Prev
                  </Button>
                  <Typography variant="body2" sx={{ mx: 1 }}>
                    Page {page} / {totalPages}
                  </Typography>
                  <Button
                    size="small"
                    onClick={() => setPage((p) => (p < totalPages ? p + 1 : p))}
                    disabled={page >= totalPages || loadingTokens}
                  >
                    Next
                  </Button>
                </Stack>
              </Stack>
            </>
          )}
        </Box>
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
