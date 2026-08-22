// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { Backdrop, CircularProgress } from "@mui/material";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { generateSupportBundle } from "@/queries/support";
import ConfirmDialog from "@/components/form/ConfirmDialog";

export default function SupportModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
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
      onClose();
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
      <Backdrop open={isGenerating} sx={{ zIndex: (t) => t.zIndex.modal + 1 }}>
        <CircularProgress color="inherit" />
      </Backdrop>

      <ConfirmDialog
        open={open}
        onClose={onClose}
        onConfirm={handleGenerate}
        title="Generate Support Bundle"
        description="The support bundle contains system diagnostics, configuration and database-derived information to help Ella Networks investigate issues. Sensitive fields (like private keys) are redacted where possible. You can inspect the downloaded archive before sharing it."
        confirmLabel="Generate"
        confirmingLabel="Generating…"
        confirmColor="success"
        fullWidth
      />
    </>
  );
}
