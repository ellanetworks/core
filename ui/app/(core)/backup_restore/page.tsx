"use client";
import React, { useState } from "react";
import { Box, Typography, Button, Alert } from "@mui/material";
import { backup, restore } from "@/queries/backup";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import Grid from "@mui/material/Grid2";

const BackupRestore = () => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });

  const handleCreate = async () => {
    try {
      const backupBlob = await backup(cookies.user_token);

      const date = new Date();
      const formattedDate = `${date.getFullYear()}_${String(date.getMonth() + 1).padStart(2, "0")}_${String(date.getDate()).padStart(2, "0")}`;
      const fileName = `ella_core_${formattedDate}.backup`;

      const url = window.URL.createObjectURL(backupBlob);
      const link = document.createElement("a");
      link.href = url;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      setAlert({ message: "Backup created successfully!", severity: "success" });
    } catch (error) {
      console.error("Error creating backup:", error);

      const errorMessage = error instanceof Error ? error.message : "An unknown error occurred";
      setAlert({
        message: `Failed to create backup: ${errorMessage}`,
        severity: "error",
      });
    }
  };

  const handleRestore = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      try {
        await restore(cookies.user_token, file);
        setAlert({
          message: "Restore completed successfully!",
          severity: "success",
        });
      } catch (error) {
        console.error("Error restoring backup:", error);

        const errorMessage = error instanceof Error ? error.message : "An unknown error occurred";
        setAlert({
          message: `Failed to restore backup: ${errorMessage}`,
          severity: "error",
        });
      }
    }
  };

  return (
    <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>
      <Typography
        variant="h4"
        component="h1"
        gutterBottom
        sx={{ textAlign: "left", marginBottom: 4 }}
      >
        Backup and Restore
      </Typography>

      {alert.severity && (
        <Alert
          severity={alert.severity}
          onClose={() => setAlert({ message: "", severity: null })}
        >
          {alert.message}
        </Alert>
      )}

      <Grid container spacing={4} justifyContent="flex-start">
        <Grid size={12}>
          <Box
            sx={{
              border: "1px solid #ccc",
              borderRadius: 4,
              padding: 4,
              width: "100%",
              marginBottom: "24px",
              margin: "0 auto",
              textAlign: "center",
            }}
          >
            <Typography variant="h5" component="h2" sx={{ marginBottom: 2 }}>
              Create a Backup
            </Typography>
            <Button
              variant="contained"
              color="success"
              onClick={handleCreate}
              sx={{ padding: "4px 16px" }}
            >
              Create
            </Button>
          </Box>
        </Grid>
        <Grid size={12}>
          <Box
            sx={{
              border: "1px solid #ccc",
              borderRadius: 4,
              padding: 4,
              width: "100%",
              margin: "0 auto",
              textAlign: "center",
            }}
          >
            <Typography variant="h5" component="h2" sx={{ marginBottom: 2 }}>
              Restore a Backup
            </Typography>
            <Button
              variant="contained"
              component="label"
              color="success"
              sx={{ padding: "4px 16px" }}
            >
              Upload File
              <input
                type="file"
                hidden
                accept=".backup"
                onChange={handleRestore}
              />
            </Button>
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default BackupRestore;
