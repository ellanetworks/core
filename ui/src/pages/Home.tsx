// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useEffect, useState } from "react";
import { Box } from "@mui/material";
import { useNavigate } from "react-router-dom";
import { getStatus } from "@/queries/status";
import ErrorAlert from "@/components/ErrorAlert";

export default function Home() {
  const navigate = useNavigate();
  const [error, setError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;

    const checkInitialization = async () => {
      try {
        const status = await getStatus();
        if (cancelled) return;
        navigate(status?.initialized ? "/login" : "/initialize", {
          replace: true,
        });
      } catch (err) {
        if (!cancelled) setError(err);
      }
    };

    setError(null);
    void checkInitialization();

    return () => {
      cancelled = true;
    };
  }, [navigate, attempt]);

  if (!error) return null;

  return (
    <Box sx={{ p: 4, maxWidth: 640, mx: "auto" }}>
      <ErrorAlert
        resource="Ella Core"
        error={error}
        onRetry={() => setAttempt((a) => a + 1)}
      />
    </Box>
  );
}
