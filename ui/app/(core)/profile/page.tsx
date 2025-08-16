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
} from "@mui/material";
import Grid from "@mui/material/Grid";
import { useCookies } from "react-cookie";
import EditMyUserPasswordModal from "@/components/EditMyUserPasswordModal";
import CreateAPITokenModal from "@/components/CreateAPITokenModal";
import DeleteIcon from "@mui/icons-material/Delete";
import { getLoggedInUser } from "@/queries/users";
import { listAPITokens, deleteAPIToken } from "@/queries/api_tokens";
import { useAuth } from "@/contexts/AuthContext";
import { User } from "@/types/types";
import { useRouter } from "next/navigation";
import EmailIcon from "@mui/icons-material/Email";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { APIToken } from "@/types/types";

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
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);

  useEffect(() => {
    if (!cookies.user_token) router.push("/login");
  }, [cookies.user_token, router]);

  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);
  const [isCreateAPITokenModalOpen, setCreateAPITokenModalOpen] =
    useState(false);

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const [loggedInUser, setLoggedInUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [apiTokens, setAPITokens] = useState<APIToken[]>([]);
  const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null);
  const [selectedTokenName, setSelectedTokenName] = useState<string | null>(
    null,
  );

  const [isConfirmationOpen, setConfirmationOpen] = useState(false);

  const fetchUser = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getLoggedInUser(cookies.user_token);
      setLoggedInUser(data);
    } catch (error) {
      console.error("Error fetching user:", error);
      setAlert({ message: "Failed to load profile info.", severity: "error" });
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  const handleEditPasswordClick = (user: User | null) => {
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

  const handleCreateAPITokenSuccess = () => {
    setCreateAPITokenModalOpen(false);
    setAlert({
      message: "API Token created successfully.",
      severity: "success",
    });
    fetchAPITokens();
  };

  const handleDeleteClick = (tokenId: number, tokenName: string) => {
    setSelectedTokenId(tokenId);
    setSelectedTokenName(tokenName);

    setConfirmationOpen(true);
  };

  const fetchAPITokens = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAPITokens(cookies.user_token);
      setAPITokens(data);
    } catch (error) {
      console.error("Error fetching API Tokens:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchAPITokens();
  }, [fetchAPITokens]);

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedTokenId) return;
    try {
      await deleteAPIToken(cookies.user_token, selectedTokenId);
      setAlert({
        message: `API Token "${selectedTokenName}" deleted successfully!`,
        severity: "success",
      });
      fetchAPITokens();
    } catch (error) {
      setAlert({
        message: `Failed to delete policy "${selectedTokenName}": ${
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
              {loading ? (
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
                  disabled={loading || !loggedInUser}
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
                  disabled={loading || !loggedInUser}
                >
                  Create Token
                </Button>
              </Box>

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
                    {loading && (
                      <>
                        {[...Array(2)].map((_, i) => (
                          <TableRow key={`sk-${i}`}>
                            <TableCell colSpan={4}>
                              <Skeleton variant="text" />
                            </TableCell>
                          </TableRow>
                        ))}
                      </>
                    )}

                    {!loading && apiTokens.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={4}>
                          <Typography variant="body2" color="text.secondary">
                            No API tokens yet. Click “Create Token” to add one.
                          </Typography>
                        </TableCell>
                      </TableRow>
                    )}

                    {!loading &&
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
                                disabled={loading}
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
