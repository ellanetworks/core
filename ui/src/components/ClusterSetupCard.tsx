// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import {
  bootstrapCluster,
  joinCluster,
  type ClusterJoinState,
} from "@/queries/cluster";

type Props = {
  state: ClusterJoinState;
  failure?: string;
  joinOnly?: boolean;
  onSubmitted: () => void;
  onCancel?: () => void;
};

const ClusterSetupCard = ({
  state,
  failure,
  joinOnly = false,
  onSubmitted,
  onCancel,
}: Props) => {
  const [mode, setMode] = useState<"idle" | "join">(joinOnly ? "join" : "idle");
  const [token, setToken] = useState("");
  const [seedAddress, setSeedAddress] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const busy = submitting || state === "joining";

  const run = async (fn: () => Promise<unknown>) => {
    setSubmitting(true);
    setError("");

    try {
      await fn();
      onSubmitted();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card variant="outlined" sx={{ maxWidth: 560 }}>
      <CardContent>
        <Typography variant="h6" gutterBottom>
          This node is not part of a cluster
        </Typography>

        {(error || failure) && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error || failure}
          </Alert>
        )}

        {mode === "idle" && (
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button
              variant="contained"
              disabled={busy}
              startIcon={busy ? <CircularProgress size={16} /> : undefined}
              onClick={() => void run(bootstrapCluster)}
            >
              Create a cluster
            </Button>
            <Button
              variant="outlined"
              disabled={busy}
              onClick={() => setMode("join")}
            >
              Join an existing cluster
            </Button>
          </Stack>
        )}

        {mode === "join" && (
          <Box sx={{ mt: 1 }}>
            <TextField
              fullWidth
              label="Join token"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              disabled={busy}
              margin="normal"
              multiline
              minRows={2}
            />
            <TextField
              fullWidth
              label="Cluster address"
              placeholder="10.0.0.1:7000"
              value={seedAddress}
              onChange={(e) => setSeedAddress(e.target.value)}
              disabled={busy}
              margin="normal"
            />

            <Alert severity="warning" sx={{ mt: 2 }}>
              Everything on this node will be replaced by the cluster&apos;s
              data.
            </Alert>

            <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
              <Button
                variant="contained"
                disabled={busy || !token.trim() || !seedAddress.trim()}
                startIcon={busy ? <CircularProgress size={16} /> : undefined}
                onClick={() =>
                  void run(() =>
                    joinCluster(token.trim(), [seedAddress.trim()]),
                  )
                }
              >
                {busy ? "Joining…" : "Join"}
              </Button>
              <Button
                disabled={busy}
                onClick={() => (joinOnly ? onCancel?.() : setMode("idle"))}
              >
                Back
              </Button>
            </Stack>
          </Box>
        )}
      </CardContent>
    </Card>
  );
};

export default ClusterSetupCard;
