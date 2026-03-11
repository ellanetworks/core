import React, { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import CloseIcon from "@mui/icons-material/Close";
import { useSnackbar } from "@/contexts/SnackbarContext";
import type { APIToken } from "@/queries/api_tokens";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import CreateAPITokenModal from "@/components/CreateAPITokenModal";

interface UserAPITokensCardProps {
  tokens: APIToken[];
  maxTokens?: number;
  /** Called after a token is successfully deleted. */
  onDeleteToken: (token: APIToken) => Promise<void>;
  /** Called with the raw token string after creation. */
  onTokenCreated: (token: string) => void;
  /** Target email when creating tokens for another user (admin view). */
  targetEmail?: string;
}

const UserAPITokensCard: React.FC<UserAPITokensCardProps> = ({
  tokens,
  maxTokens = 12,
  onDeleteToken,
  onTokenCreated,
  targetEmail,
}) => {
  const { showSnackbar } = useSnackbar();
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [newToken, setNewToken] = useState<string | null>(null);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [selectedToken, setSelectedToken] = useState<APIToken | null>(null);

  const copyToken = async () => {
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(newToken ?? "");
      showSnackbar("Copied to clipboard.", "success");
    } catch {
      showSnackbar("Failed to copy API token.", "error");
    }
  };

  const handleDeleteClick = (token: APIToken) => {
    setSelectedToken(token);
    setDeleteConfirmOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setDeleteConfirmOpen(false);
    if (!selectedToken) return;
    try {
      await onDeleteToken(selectedToken);
    } finally {
      setSelectedToken(null);
    }
  };

  const handleTokenCreated = (token: string) => {
    setNewToken(token);
    onTokenCreated(token);
  };

  return (
    <>
      <Card variant="outlined">
        <CardContent>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 2,
            }}
          >
            <Typography variant="h6">API Tokens</Typography>
            <Tooltip
              title={
                tokens.length >= maxTokens
                  ? `Maximum of ${maxTokens} API tokens reached.`
                  : ""
              }
            >
              <span>
                <Button
                  variant="contained"
                  color="success"
                  size="small"
                  onClick={() => {
                    setNewToken(null);
                    setCreateModalOpen(true);
                  }}
                  disabled={tokens.length >= maxTokens}
                >
                  Create Token
                </Button>
              </span>
            </Tooltip>
          </Box>

          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {targetEmail
              ? "Manage API tokens for this user. Tokens authenticate programmatically with Ella Core and inherit the user's permissions."
              : "Manage your API tokens to authenticate programmatically with Ella Core. Your API token will have the same permissions as your user account. Actions performed with the token will be logged under your user account."}
          </Typography>

          {newToken && (
            <Alert
              severity="success"
              variant="outlined"
              sx={{ alignItems: "center", mb: 2 }}
              action={
                <Stack direction="row" spacing={1}>
                  <IconButton
                    aria-label="copy token"
                    size="small"
                    onClick={copyToken}
                    title="Copy"
                  >
                    <ContentCopyIcon fontSize="inherit" />
                  </IconButton>
                  <IconButton
                    aria-label="dismiss"
                    size="small"
                    onClick={() => setNewToken(null)}
                    title="Dismiss"
                  >
                    <CloseIcon fontSize="inherit" />
                  </IconButton>
                </Stack>
              }
            >
              <Typography variant="body2" sx={{ mb: 0.5 }}>
                Copy your API token now. You won't be able to see it again!
              </Typography>
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  wordBreak: "break-all",
                  userSelect: "all",
                }}
              >
                {newToken}
              </Typography>
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
                {tokens.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4}>
                      <Typography variant="body2" color="text.secondary">
                        No API tokens yet. Click "Create Token" to add one.
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}

                {tokens.map((token) => {
                  const isExpired = token.expires_at
                    ? new Date(token.expires_at).getTime() < Date.now()
                    : false;

                  return (
                    <TableRow key={token.id}>
                      <TableCell>{token.name}</TableCell>
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
                          <Chip label="Active" color="success" size="small" />
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {token.expires_at
                            ? new Date(token.expires_at).toDateString()
                            : "Never"}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <IconButton
                          aria-label="delete"
                          size="small"
                          onClick={() => handleDeleteClick(token)}
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

      {isCreateModalOpen && (
        <CreateAPITokenModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={handleTokenCreated}
          targetEmail={targetEmail}
        />
      )}
      {isDeleteConfirmOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteConfirmOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the API Token "${selectedToken?.name}"? This action cannot be undone.`}
        />
      )}
    </>
  );
};

export default UserAPITokensCard;
