import React, { useRef, useState } from "react";
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
import Grid from "@mui/material/Grid";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import theme from "@/utils/theme";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const headerStyles = {
  backgroundColor: theme.palette.backgroundSubtle,
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

  const pageDescription =
    "Create and download a full backup of Ella Core, or restore from a .backup file. Take regular backups to ensure you can recover your data in case of a hardware failure or data loss.";

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
          zIndex: (theme) => theme.zIndex.modal + 1,
          color: "#fff",
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
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          opacity: isRestoring ? 0.6 : 1,
          pointerEvents: isRestoring ? "none" : "auto",
        }}
      >
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            mx: "auto",
            px: PAGE_PADDING_X,
            mb: 3,
            display: "flex",
            flexDirection: "column",
            gap: 2,
          }}
        >
          <Typography variant="h4">Backup & Restore</Typography>
          <Typography variant="body1" color="text.secondary">
            {pageDescription}
          </Typography>
        </Box>

        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            mx: "auto",
            px: PAGE_PADDING_X,
          }}
        >
          <Grid container spacing={4} justifyContent="flex-start">
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
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
                  <Typography variant="body2" color="text.secondary">
                    Generate and download a snapshot of your Ella Core
                    configuration and data. You can then use this file to
                    restore your system if needed.
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

            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
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
                  <Typography variant="body2" color="text.secondary">
                    Upload a previously created backup file to restore Ella Core
                    to a previous state. This will overwrite your current
                    configuration and data.
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
                </CardContent>
              </Card>
            </Grid>
          </Grid>
        </Box>
      </Box>

      <Dialog
        open={isConfirmOpen}
        onClose={handleCancelRestore}
        aria-labelledby="restore-confirm-title"
        aria-describedby="restore-confirm-description"
      >
        <DialogTitle id="restore-confirm-title">Confirm Restore</DialogTitle>
        <DialogContent dividers>
          <Alert severity="warning" sx={{ mb: 2 }}>
            This operation will overwrite all current data and cannot be undone.
          </Alert>
          <DialogContentText id="restore-confirm-description">
            Are you sure you want to restore from{" "}
            <strong>{pendingFile?.name}</strong>? All existing configuration and
            data will be replaced with the contents of this backup file.
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
