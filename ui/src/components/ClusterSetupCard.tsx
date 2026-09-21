// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useEffect, useRef, useState, type ReactNode } from "react";
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
  getClusterJoinStatus,
  joinCluster,
  type ClusterJoinState,
} from "@/queries/cluster";
import ArrowForwardIcon from "@mui/icons-material/ArrowForward";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { ApiError } from "@/queries/utils";

type Props = {
  state: ClusterJoinState;
  failure?: string;
  joinOnly?: boolean;
  disabledReason?: ReactNode;
  onSubmitted: () => void;
  onCancel?: () => void;
  onJoinClick?: () => void;
};

type Pending = "join" | "create";

const WATCH_INTERVAL_MS = 1000;
const WATCH_TIMEOUT_MS = 180000;

const TIMEOUT_MESSAGE: Record<Pending, string> = {
  join: "This node is still trying to join. Check that the cluster address is reachable and that the token was minted for this node.",
  create: "This node is still trying to create the cluster.",
};

const PROGRESS_MESSAGE: Record<Pending, string> = {
  join: "Joining the cluster. This node is contacting the cluster address and copying its data, which can take a minute.",
  create: "Creating the cluster on this node.",
};

const HEADING: Record<Pending, string> = {
  join: "Joining a cluster",
  create: "Creating a cluster",
};

const ClusterSetupCard = ({
  state,
  failure,
  joinOnly = false,
  disabledReason,
  onSubmitted,
  onCancel,
  onJoinClick,
}: Props) => {
  const [mode, setMode] = useState<"idle" | "join">(joinOnly ? "join" : "idle");
  const [token, setToken] = useState("");
  const [seedAddress, setSeedAddress] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);

  const onSubmittedRef = useRef(onSubmitted);

  useEffect(() => {
    onSubmittedRef.current = onSubmitted;
  });

  useEffect(() => {
    if (pending === null) return;

    let cancelled = false;
    const deadline = Date.now() + WATCH_TIMEOUT_MS;

    const giveUp = (reason: string) => {
      if (cancelled) return;
      setError(reason);
      setPending(null);
    };

    const watch = async () => {
      for (;;) {
        if (cancelled) return;

        try {
          const status = await getClusterJoinStatus();
          if (cancelled) return;

          if (status.state === "joined") {
            setPending(null);
            onSubmittedRef.current();
            return;
          }

          if (status.state === "waiting" && status.error) {
            giveUp(status.error);
            return;
          }
        } catch (err) {
          if (cancelled) return;

          if (!(err instanceof ApiError) || !err.retryable) {
            giveUp(err instanceof Error ? err.message : String(err));
            return;
          }
        }

        if (Date.now() > deadline) {
          giveUp(TIMEOUT_MESSAGE[pending]);
          return;
        }

        await new Promise((resolve) => setTimeout(resolve, WATCH_INTERVAL_MS));
      }
    };

    void watch();

    return () => {
      cancelled = true;
    };
  }, [pending]);

  const blocked = disabledReason !== undefined;
  const working = submitting || pending !== null || state === "joining";
  const busy = blocked || working;

  const run = async (what: Pending, fn: () => Promise<unknown>) => {
    setSubmitting(true);
    setError("");

    try {
      await fn();
      setPending(what);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const body = (
    <>
      {!(joinOnly && pending === null) && (
        <Typography variant="h6" gutterBottom>
          {pending === null
            ? "This node is not part of a cluster"
            : HEADING[pending]}
        </Typography>
      )}

      {blocked && (
        <Alert severity="info" sx={{ mb: 2 }}>
          {disabledReason}
        </Alert>
      )}

      {pending === null && (error || failure) && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error || failure}
        </Alert>
      )}

      {pending !== null && (
        <Stack
          direction="row"
          spacing={2}
          sx={{ mt: 2, alignItems: "center" }}
          component="output"
        >
          <CircularProgress size={20} />
          <Typography variant="body2">{PROGRESS_MESSAGE[pending]}</Typography>
        </Stack>
      )}

      {pending === null && mode === "idle" && (
        <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
          <Button
            variant="contained"
            disabled={busy}
            startIcon={working ? <CircularProgress size={16} /> : undefined}
            onClick={() => void run("create", bootstrapCluster)}
          >
            Create a cluster
          </Button>
          <Button
            variant="outlined"
            disabled={busy}
            endIcon={<ArrowForwardIcon />}
            onClick={() => (onJoinClick ? onJoinClick() : setMode("join"))}
          >
            Join an existing cluster
          </Button>
        </Stack>
      )}

      {pending === null && mode === "join" && (
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

          <Button
            variant="contained"
            color="success"
            fullWidth
            sx={{ mt: 2 }}
            disabled={busy || !token.trim() || !seedAddress.trim()}
            startIcon={working ? <CircularProgress size={16} /> : undefined}
            onClick={() =>
              void run("join", () =>
                joinCluster(token.trim(), [seedAddress.trim()]),
              )
            }
          >
            {working ? "Joining…" : "Join"}
          </Button>

          <Button
            fullWidth
            sx={{ mt: 2 }}
            startIcon={<ArrowBackIcon />}
            disabled={busy}
            onClick={() => (joinOnly ? onCancel?.() : setMode("idle"))}
          >
            Back
          </Button>
        </Box>
      )}
    </>
  );

  if (joinOnly) {
    return body;
  }

  return (
    <Card variant="outlined" sx={{ maxWidth: 560 }}>
      <CardContent>{body}</CardContent>
    </Card>
  );
};

export default ClusterSetupCard;
