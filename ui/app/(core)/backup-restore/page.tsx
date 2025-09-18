"use client";
import React, { useState } from "react";
import {
  Box,
  Typography,
  Button,
  Alert,
  Collapse,
  Card,
  CardHeader,
  CardContent,
} from "@mui/material";
import { backup, restore } from "@/queries/backup";
import Grid from "@mui/material/Grid";
import { useAuth } from "@/contexts/AuthContext";

const MAX_WIDTH = 1400;

const headerStyles = {
  backgroundColor: "#F5F5F5",
  color: "#000000ff",
  borderTopLeftRadius: 12,
  borderTopRightRadius: 12,
  "& .MuiCardHeader-title": { color: "#000000ff" },
};

const BackupRestore = () => {
  const { accessToken, authReady } = useAuth();
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const pageDescription =
    "Create and download a full backup of Ella Core, or restore from a .backup file. Take regular backups to ensure you can recover your data in case of a hardware failure or data loss.";

  const handleCreate = async () => {
    if (!authReady || !accessToken) {
      setAlert({
        message: "Authentication not ready. Please try again later.",
        severity: "error",
      });
      return;
    }
    try {
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

      setAlert({
        message: "Backup created successfully!",
        severity: "success",
      });
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "An unknown error occurred";
      setAlert({
        message: `Failed to create backup: ${errorMessage}`,
        severity: "error",
      });
    }
  };

  const handleRestore = async (event: React.ChangeEvent<HTMLInputElement>) => {
    if (!authReady || !accessToken) {
      setAlert({
        message: "Authentication not ready. Please try again later.",
        severity: "error",
      });
      return;
    }
    const file = event.target.files?.[0];
    if (!file) return;
    try {
      await restore(accessToken, file);
      setAlert({
        message: "Restore completed successfully!",
        severity: "success",
      });
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "An unknown error occurred";
      setAlert({
        message: `Failed to restore backup: ${errorMessage}`,
        severity: "error",
      });
    }
  };

  return (
    <Box
      sx={{
        pt: 6,
        pb: 4,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
      }}
    >
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Collapse in={!!alert.severity}>
          <Alert
            severity={alert.severity || "success"}
            onClose={() => setAlert({ message: "", severity: null })}
            sx={{ mb: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>

      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          px: { xs: 2, sm: 4 },
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

      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Grid container spacing={4} justifyContent="flex-start">
          <Grid size={{ xs: 12, sm: 6, md: 4 }}>
            <Card
              sx={{
                height: "100%",
                display: "flex",
                flexDirection: "column",
                borderRadius: 3, // 12px
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
                  configuration and data. You can then use this file to restore
                  your system if needed.
                </Typography>

                <Box sx={{ flexGrow: 1 }} />
                <Box sx={{ display: "flex", justifyContent: "center" }}>
                  <Button
                    variant="contained"
                    color="primary"
                    onClick={handleCreate}
                  >
                    Create Backup
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
                  to a previous state. Be aware that this action will overwrite
                  your current configuration and data.
                </Typography>

                <Box sx={{ flexGrow: 1 }} />

                <Box sx={{ display: "flex", justifyContent: "center" }}>
                  <Button variant="contained" component="label" color="primary">
                    Upload File
                    <input
                      type="file"
                      hidden
                      accept=".backup"
                      onChange={handleRestore}
                    />
                  </Button>
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      </Box>
    </Box>
  );
};

export default BackupRestore;
