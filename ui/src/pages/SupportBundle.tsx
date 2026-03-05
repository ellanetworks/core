import React, { useState } from "react";
import {
  Box,
  Typography,
  Button,
  Card,
  CardHeader,
  CardContent,
  Backdrop,
  CircularProgress,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { generateSupportBundle } from "@/queries/support";

const MAX_WIDTH = 1400;

const headerStyles = {
  backgroundColor: "#F5F5F5",
  color: "#000000ff",
  borderTopLeftRadius: 12,
  borderTopRightRadius: 12,
  "& .MuiCardHeader-title": { color: "#000000ff" },
};

const SupportBundlePage = () => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();

  const [isGenerating, setIsGenerating] = useState(false);

  const handleGenerate = async () => {
    if (!authReady || !accessToken) {
      showSnackbar(
        "Authentication not ready. Please try again later.",
        "error",
      );
      return;
    }

    try {
      setIsGenerating(true);
      const blob = await generateSupportBundle(accessToken);

      const date = new Date();
      const formattedDate = `${date.getFullYear()}_${String(
        date.getMonth() + 1,
      ).padStart(2, "0")}_${String(date.getDate()).padStart(2, "0")}_${String(
        date.getHours(),
      ).padStart(2, "0")} ${String(date.getMinutes()).padStart(2, "0")}`;
      const fileName = `ella-support-${formattedDate}.tar.gz`;

      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      showSnackbar("Support bundle generated and downloaded.", "success");
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "An unknown error occurred";
      showSnackbar(
        `Failed to generate support bundle: ${errorMessage}`,
        "error",
      );
    } finally {
      setIsGenerating(false);
    }
  };

  return (
    <>
      <Backdrop
        open={isGenerating}
        sx={{
          zIndex: (theme) => theme.zIndex.modal + 1,
          color: "#fff",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <CircularProgress />
        <Typography variant="h6">Generating support bundle…</Typography>
        <Typography variant="body2">
          This can take a moment. The download will start automatically when
          ready.
        </Typography>
      </Backdrop>

      <Box
        sx={{
          pt: 6,
          pb: 4,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
        }}
      >
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
          <Typography variant="h4">Support Bundle</Typography>
        </Box>

        <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
          <Grid container spacing={4} justifyContent="center">
            <Grid size={{ xs: 12, sm: 8, md: 6 }}>
              <Card
                sx={{
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  borderRadius: 3,
                  boxShadow: 2,
                }}
              >
                <CardHeader
                  title="Generate a Support Bundle"
                  sx={headerStyles}
                />
                <CardContent
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    gap: 1.5,
                    flexGrow: 1,
                  }}
                >
                  <Typography variant="body2" color="text.secondary">
                    The support bundle contains system diagnostics,
                    configuration and database-derived information to help Ella
                    Networks investigate issues. Sensitive fields (like private
                    keys) are redacted where possible. You can inspect the
                    downloaded archive before sharing it.
                  </Typography>

                  <Box sx={{ flexGrow: 1 }} />
                  <Box sx={{ display: "flex", justifyContent: "center" }}>
                    <Button
                      variant="contained"
                      color="primary"
                      onClick={handleGenerate}
                      disabled={isGenerating}
                    >
                      {isGenerating ? "Generating…" : "Generate Support Bundle"}
                    </Button>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          </Grid>
        </Box>
      </Box>
    </>
  );
};

export default SupportBundlePage;
