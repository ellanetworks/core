// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useRef, useState } from "react";
import PageTitle from "@/components/PageTitle";
import {
  Alert,
  Box,
  Typography,
  Button,
  Card,
  CardHeader,
  CardContent,
  Backdrop,
  CircularProgress,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
} from "@mui/material";
import { backup, restore } from "@/queries/backup";
import QueryState from "@/components/QueryState";
import Grid from "@mui/material/Grid";
import { useAuth } from "@/contexts/AuthContext";
import { useStatusQuery } from "@/hooks/useStatus";
import { useSnackbar } from "@/contexts/SnackbarContext";
import type { Theme } from "@mui/material/styles";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import { PRODUCT } from "@/utils/product";

const CARDS_MAX_WIDTH = 900;

const headerStyles = {
  backgroundColor: (theme: Theme) => theme.palette.backgroundSubtle,
  color: "text.primary",
  borderTopLeftRadius: 12,
  borderTopRightRadius: 12,
  "& .MuiCardHeader-title": { color: "text.primary" },
};

const BackupRestore = () => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();

  const [isBackingUp, setIsBackingUp] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const statusQuery = useStatusQuery();

  const pageDescription = `Take regular backups of ${PRODUCT.name} so you can recover your data after a hardware failure or data loss.`;

  const handleCreate = async () => {
    if (!authReady || !accessToken) {
      showSnackbar(
        "Authentication not ready. Please try again later.",
        "error",
      );
      return;
    }

    if (isRestoring) {
      showSnackbar(
        "Cannot create a backup while a restore is in progress.",
        "error",
      );
      return;
    }

    try {
      setIsBackingUp(true);
      const backupBlob = await backup(accessToken);

      const date = new Date();
      const formattedDate = `${date.getFullYear()}_${String(
        date.getMonth() + 1,
      ).padStart(2, "0")}_${String(date.getDate()).padStart(2, "0")}`;
      const fileName = `ella_core_${formattedDate}.backup`;

      const url = window.URL.createObjectURL(backupBlob);
      const link = document.createElement("a");
      link.href = url;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      showSnackbar("Backup created successfully.", "success");
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "An unknown error occurred";
      showSnackbar(`Failed to create backup: ${errorMessage}`, "error");
    } finally {
      setIsBackingUp(false);
    }
  };

  const handleRestore = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (!authReady || !accessToken) {
      showSnackbar(
        "Authentication not ready. Please try again later.",
        "error",
      );
      return;
    }

    const file = event.target.files?.[0];
    if (!file) return;

    event.target.value = "";
    setPendingFile(file);
    setIsConfirmOpen(true);
  };

  const handleConfirmRestore = async () => {
    if (!accessToken || !pendingFile) return;
    setIsConfirmOpen(false);

    try {
      setIsRestoring(true);
      showSnackbar(
        "Restore is in progress. This may take a few minutes. Please do not close this page or navigate away.",
        "info",
      );

      await restore(accessToken, pendingFile);

      showSnackbar(
        "Restore completed successfully. You may need to refresh the page.",
        "success",
      );
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "An unknown error occurred";
      showSnackbar(`Failed to restore backup: ${errorMessage}`, "error");
    } finally {
      setIsRestoring(false);
      setPendingFile(null);
    }
  };

  const handleCancelRestore = () => {
    setIsConfirmOpen(false);
    setPendingFile(null);
  };

  const actionsDisabled = isRestoring;

  return (
    <>
      <Backdrop
        open={isRestoring}
        sx={{
          zIndex: (t) => t.zIndex.modal + 1,
          color: "common.white",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <CircularProgress />
        <Typography variant="h6">Restoring backup…</Typography>
        <Typography variant="body2">
          This can take a few minutes. Please do not close this tab or make
          changes.
        </Typography>
      </Backdrop>

      <Box
        sx={{
          pt: 6,
          pb: 4,
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
          opacity: isRestoring ? 0.6 : 1,
          pointerEvents: isRestoring ? "none" : "auto",
        }}
      >
        <Box
          sx={{
            mb: 3,
            display: "flex",
            flexDirection: "column",
            gap: 2,
          }}
        >
          <PageTitle title="Backup & Restore" />
          <Typography variant="body1" color="textSecondary">
            {pageDescription}
          </Typography>
        </Box>

        <Grid container spacing={3} sx={{ maxWidth: CARDS_MAX_WIDTH }}>
          <Grid size={{ xs: 12, sm: 6 }}>
            <Card
              sx={{
                height: "100%",
                display: "flex",
                flexDirection: "column",
                borderRadius: 3,
                boxShadow: 2,
              }}
            >
              <CardHeader title="Create a Backup" sx={headerStyles} />
              <CardContent
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 1.5,
                  flexGrow: 1,
                }}
              >
                <Typography variant="body2" color="textSecondary">
                  Download a snapshot of your {PRODUCT.name} configuration and
                  data, which you can later use to restore your system. The
                  archive <strong>contains sensitive secrets</strong>: store and
                  transfer it encrypted, and treat it as an admin credential.
                </Typography>

                <Box sx={{ flexGrow: 1 }} />
                <Box sx={{ display: "flex", justifyContent: "center" }}>
                  <Button
                    variant="contained"
                    color="primary"
                    onClick={handleCreate}
                    disabled={actionsDisabled || isBackingUp}
                  >
                    {isBackingUp ? "Creating Backup…" : "Create Backup"}
                  </Button>
                </Box>
              </CardContent>
            </Card>
          </Grid>

          <Grid size={{ xs: 12, sm: 6 }}>
            <Card
              sx={{
                height: "100%",
                display: "flex",
                flexDirection: "column",
                borderRadius: 3,
                boxShadow: 2,
              }}
            >
              <CardHeader title="Restore a Backup" sx={headerStyles} />
              <CardContent
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 1.5,
                  flexGrow: 1,
                }}
              >
                <QueryState query={statusQuery} resource="system status">
                  {(status) =>
                    status.cluster?.enabled ? (
                      <>
                        <Alert severity="info">
                          Online restore is disabled in HA mode. Clustered
                          deployments recover by seeding a fresh node from the
                          backup archive via the <code>restore.bundle</code>{" "}
                          drop-in path.
                        </Alert>
                        <Typography variant="body2" color="textSecondary">
                          See the backup and restore documentation for the
                          step-by-step disaster-recovery procedure.
                        </Typography>
                        <Box sx={{ flexGrow: 1 }} />
                      </>
                    ) : (
                      <>
                        <Typography variant="body2" color="textSecondary">
                          Upload a previously created backup file to restore{" "}
                          {PRODUCT.name} to a previous state.{" "}
                          <strong>
                            This overwrites your current configuration and data.
                          </strong>
                        </Typography>

                        <Box sx={{ flexGrow: 1 }} />

                        <Box sx={{ display: "flex", justifyContent: "center" }}>
                          <Button
                            variant="contained"
                            color="primary"
                            onClick={() => fileInputRef.current?.click()}
                            disabled={actionsDisabled}
                          >
                            {isRestoring ? "Restoring…" : "Upload File"}
                          </Button>
                          <input
                            ref={fileInputRef}
                            type="file"
                            hidden
                            accept=".backup"
                            onChange={handleRestore}
                          />
                        </Box>
                      </>
                    )
                  }
                </QueryState>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      </Box>

      <Dialog
        open={isConfirmOpen}
        onClose={handleCancelRestore}
        aria-labelledby="restore-confirm-title"
        aria-describedby="restore-confirm-description"
      >
        <DialogTitle id="restore-confirm-title">Confirm Restore</DialogTitle>
        <DialogContent dividers>
          <DialogContentText id="restore-confirm-description">
            Restore from <strong>{pendingFile?.name}</strong>? All existing
            configuration and data will be replaced with the contents of this
            backup file. <strong>This cannot be undone.</strong>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCancelRestore}>Cancel</Button>
          <Button
            variant="contained"
            color="error"
            onClick={handleConfirmRestore}
          >
            Restore
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

export default BackupRestore;
