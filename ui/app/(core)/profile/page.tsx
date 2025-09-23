"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Alert,
  Typography,
  Card,
  CardHeader,
  CardContent,
  TableContainer,
  Table,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  IconButton,
  Paper,
  Button,
  Chip,
  Skeleton,
  Stack,
  Select,
  MenuItem,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import EditMyUserPasswordModal from "@/components/EditMyUserPasswordModal";
import CreateAPITokenModal from "@/components/CreateAPITokenModal";
import DeleteIcon from "@mui/icons-material/Delete";
import { getLoggedInUser, type APIUser } from "@/queries/users";
import {
  listAPITokens,
  deleteAPIToken,
  type APIToken,
  type ListAPITokensResponse,
} from "@/queries/api_tokens";
import { useAuth } from "@/contexts/AuthContext";
import { useRouter } from "next/navigation";
import EmailIcon from "@mui/icons-material/Email";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import CloseIcon from "@mui/icons-material/Close";

const MAX_WIDTH = 1200;

const headerStyles = {
  backgroundColor: "#F5F5F5",
  color: "#000000ff",
  borderTopLeftRadius: 12,
  borderTopRightRadius: 12,
  "& .MuiCardHeader-title": { color: "#000000ff" },
  "& .MuiIconButton-root": { color: "#000000ff" },
};

export default function Profile() {
  const router = useRouter();
  const { role, accessToken, authReady } = useAuth();

  useEffect(() => {
    if (authReady && !accessToken) router.push("/login");
  }, [authReady, accessToken, router]);

  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);
  const [isCreateAPITokenModalOpen, setCreateAPITokenModalOpen] =
    useState(false);

  const [justCreatedToken, setJustCreatedToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const handleCreateAPITokenSuccess = (token: string) => {
    setCreateAPITokenModalOpen(false);
    setJustCreatedToken(token);
    setPage(1);
    fetchAPITokens(1, perPage);
  };

  const copyToken = async () => {
    try {
      await navigator.clipboard.writeText(justCreatedToken ?? "");
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {}
  };

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const [loggedInUser, setLoggedInUser] = useState<APIUser | null>(null);
  const [loadingUser, setLoadingUser] = useState(true);

  // --- API tokens state (paginated) ---
  const [apiTokens, setAPITokens] = useState<APIToken[]>([]);
  const [loadingTokens, setLoadingTokens] = useState(true);
  const [page, setPage] = useState<number>(1);
  const [perPage, setPerPage] = useState<number>(10);
  const [totalCount, setTotalCount] = useState<number>(0);

  const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null);
  const [selectedTokenName, setSelectedTokenName] = useState<string | null>(
    null,
  );
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);

  const fetchUser = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      setLoadingUser(true);
      const data = await getLoggedInUser(accessToken);
      setLoggedInUser(data);
    } catch (error) {
      console.error("Error fetching user:", error);
      setAlert({ message: "Failed to load profile info.", severity: "error" });
    } finally {
      setLoadingUser(false);
    }
  }, [accessToken, authReady]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  const handleEditPasswordClick = (user: APIUser | null) => {
    if (!user) return;
    setEditPasswordModalOpen(true);
  };

  const handleOpenCreateAPITokenModal = () => setCreateAPITokenModalOpen(true);

  const handlePasswordSuccess = () => {
    setEditPasswordModalOpen(false);
    setAlert({
      message: "Password updated successfully.",
      severity: "success",
    });
  };

  const handleDeleteClick = (tokenId: number, tokenName: string) => {
    setSelectedTokenId(tokenId);
    setSelectedTokenName(tokenName);
    setConfirmationOpen(true);
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
      } catch (error) {
        console.error("Error fetching API Tokens:", error);
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

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedTokenId || !accessToken) return;
    try {
      await deleteAPIToken(accessToken, selectedTokenId);
      setAlert({
        message: `API Token "${selectedTokenName}" deleted successfully!`,
        severity: "success",
      });
      // If this was the last item on the page and not the first page, go back one page
      const remainingOnPage = apiTokens.length - 1;
      if (remainingOnPage <= 0 && page > 1) {
        const newPage = page - 1;
        setPage(newPage);
        fetchAPITokens(newPage, perPage);
      } else {
        fetchAPITokens(page, perPage);
      }
    } catch (error) {
      setAlert({
        message: `Failed to delete token "${selectedTokenName}": ${
          error instanceof Error ? error.message : "Unknown error"
        }`,
        severity: "error",
      });
    } finally {
      setSelectedTokenId(null);
      setSelectedTokenName(null);
    }
  };

  const descriptionText = "Manage how you authenticate with Ella Core.";

  const totalPages = Math.max(1, Math.ceil(totalCount / perPage));
  const from = totalCount === 0 ? 0 : (page - 1) * perPage + 1;
  const to = Math.min(page * perPage, totalCount);

  return (
    <Box sx={{ p: { xs: 2, sm: 3, md: 4 }, maxWidth: MAX_WIDTH, mx: "auto" }}>
      <Typography variant="h4" sx={{ mb: 1 }}>
        My Profile
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mb: 3 }}>
        {descriptionText}
      </Typography>

      {alert.severity && (
        <Box sx={{ mb: 3 }}>
          <Alert
            severity={alert.severity}
            onClose={() => setAlert({ message: "", severity: null })}
          >
            {alert.message}
          </Alert>
        </Box>
      )}

      <Grid container spacing={3} alignItems="stretch">
        <Grid size={{ xs: 12, sm: 12, md: 4 }}>
          <Card sx={{ height: "100%", borderRadius: 3, boxShadow: 2 }}>
            <CardHeader title="Account" sx={headerStyles} />
            <CardContent>
              {loadingUser ? (
                <>
                  <Stack
                    direction="row"
                    spacing={2}
                    alignItems="center"
                    sx={{ mb: 2 }}
                  >
                    <Skeleton variant="circular" width={56} height={56} />
                    <Box sx={{ flex: 1 }}>
                      <Skeleton variant="text" width="80%" />
                      <Skeleton variant="text" width="50%" />
                    </Box>
                  </Stack>
                  <Skeleton variant="rectangular" height={20} sx={{ mb: 1 }} />
                  <Skeleton variant="rectangular" height={20} width="60%" />
                </>
              ) : (
                <>
                  <Stack
                    direction="row"
                    spacing={2}
                    alignItems="center"
                    sx={{ mb: 2 }}
                  >
                    <Box sx={{ minWidth: 0 }}>
                      <Stack
                        direction="row"
                        alignItems="center"
                        spacing={1}
                        sx={{ mb: 0.5 }}
                      >
                        <EmailIcon fontSize="small" />
                        <Typography
                          variant="body1"
                          noWrap
                          title={loggedInUser?.email}
                          sx={{ maxWidth: 220 }}
                        >
                          {loggedInUser?.email || "—"}
                        </Typography>
                      </Stack>
                      <Chip
                        size="small"
                        label={role || "User"}
                        color="default"
                        variant="outlined"
                        sx={{ mt: 0.5 }}
                      />
                    </Box>
                  </Stack>
                </>
              )}
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 12, md: 8 }}>
          <Card sx={{ height: "100%", borderRadius: 3, boxShadow: 2 }}>
            <CardHeader title="Password" sx={headerStyles} />
            <CardContent
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 2,
                flexGrow: 1,
              }}
            >
              <Typography variant="body2" color="text.secondary">
                Keep your account secure by using a strong password and updating
                it periodically.
              </Typography>

              <Box sx={{ display: "flex", justifyContent: "flex-start" }}>
                <Button
                  variant="contained"
                  onClick={() => handleEditPasswordClick(loggedInUser)}
                  disabled={loadingUser || !loggedInUser}
                >
                  Change Password
                </Button>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 12, md: 12 }}>
          <Card sx={{ height: "100%", borderRadius: 3, boxShadow: 2 }}>
            <CardHeader title="API Tokens" sx={headerStyles} />
            <CardContent
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 2,
                flexGrow: 1,
              }}
            >
              <Typography variant="body2" color="text.secondary">
                Manage your API tokens to authenticate programmatically with
                Ella Core. Your API token will have the same permissions as your
                user account. Actions performed with the token will be logged
                under your user account.
              </Typography>
              <Box sx={{ display: "flex", justifyContent: "flex-start" }}>
                <Button
                  variant="contained"
                  color="success"
                  onClick={handleOpenCreateAPITokenModal}
                  disabled={loadingUser || !loggedInUser}
                >
                  Create Token
                </Button>
              </Box>

              {justCreatedToken && (
                <Alert
                  severity="success"
                  variant="outlined"
                  sx={{ alignItems: "center" }}
                  action={
                    <Stack direction="row" spacing={1}>
                      <IconButton
                        aria-label="copy token"
                        size="small"
                        onClick={copyToken}
                        title={copied ? "Copied!" : "Copy"}
                      >
                        <ContentCopyIcon fontSize="inherit" />
                      </IconButton>
                      <IconButton
                        aria-label="dismiss"
                        size="small"
                        onClick={() => setJustCreatedToken(null)}
                        title="Dismiss"
                      >
                        <CloseIcon fontSize="inherit" />
                      </IconButton>
                    </Stack>
                  }
                >
                  <Typography variant="body2" sx={{ mb: 0.5 }}>
                    Copy your API token now. You won’t be able to see it again!
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{
                      fontFamily: "monospace",
                      wordBreak: "break-all",
                      userSelect: "all",
                    }}
                  >
                    {justCreatedToken}
                  </Typography>
                  {copied && (
                    <Typography variant="caption" sx={{ ml: 0.5 }}>
                      Copied!
                    </Typography>
                  )}
                </Alert>
              )}

              <TableContainer component={Paper}>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Name</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Expiry</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {loadingTokens && (
                      <>
                        {[...Array(3)].map((_, i) => (
                          <TableRow key={`sk-${i}`}>
                            <TableCell colSpan={4}>
                              <Skeleton variant="text" />
                            </TableCell>
                          </TableRow>
                        ))}
                      </>
                    )}

                    {!loadingTokens && apiTokens.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={4}>
                          <Typography variant="body2" color="text.secondary">
                            No API tokens yet. Click “Create Token” to add one.
                          </Typography>
                        </TableCell>
                      </TableRow>
                    )}

                    {!loadingTokens &&
                      apiTokens.map((t) => {
                        const isExpired = t.expires_at
                          ? new Date(t.expires_at).getTime() < Date.now()
                          : false;

                        return (
                          <TableRow key={t.id}>
                            <TableCell>{t.name}</TableCell>
                            <TableCell>
                              {isExpired ? (
                                <Chip
                                  label="Expired"
                                  size="small"
                                  sx={{
                                    backgroundColor: "#C69026",
                                    color: "#fff",
                                  }}
                                />
                              ) : (
                                <Chip
                                  label="Active"
                                  color="success"
                                  size="small"
                                />
                              )}
                            </TableCell>
                            <TableCell>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                              >
                                {t.expires_at
                                  ? new Date(t.expires_at).toDateString()
                                  : "Never"}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              <IconButton
                                aria-label="delete"
                                size="small"
                                onClick={() => handleDeleteClick(t.id, t.name)}
                                disabled={loadingTokens}
                              >
                                <DeleteIcon color="primary" />
                              </IconButton>
                            </TableCell>
                          </TableRow>
                        );
                      })}
                  </TableBody>
                </Table>
              </TableContainer>

              <Stack
                direction={{ xs: "column", sm: "row" }}
                spacing={2}
                alignItems={{ xs: "stretch", sm: "center" }}
                justifyContent="space-between"
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
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {isEditPasswordModalOpen && (
        <EditMyUserPasswordModal
          open
          onClose={() => setEditPasswordModalOpen(false)}
          onSuccess={handlePasswordSuccess}
        />
      )}
      {isCreateAPITokenModalOpen && (
        <CreateAPITokenModal
          open
          onClose={() => setCreateAPITokenModalOpen(false)}
          onSuccess={handleCreateAPITokenSuccess}
        />
      )}
      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the API Token "${selectedTokenName}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
}
