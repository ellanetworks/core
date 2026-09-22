// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  IconButton,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import {
  getClusterJoinStatus,
  joinCluster,
  type ClusterJoinState,
} from "@/queries/cluster";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import CloseIcon from "@mui/icons-material/Close";
import { ApiError } from "@/queries/utils";
import { useSnackbar } from "@/contexts/SnackbarContext";

type Props = {
  state: ClusterJoinState;
  failure?: string;
  joinOnly?: boolean;
  disabledReason?: ReactNode;
  onSubmitted: () => void;
  onCancel?: () => void;
};

type Pending = "join";

const WATCH_INTERVAL_MS = 1000;
const WATCH_TIMEOUT_MS = 180000;

const TIMEOUT_MESSAGE: Record<Pending, string> = {
  join: "This node is still trying to join. Check that the cluster address is reachable and that the token was minted for this node.",
};

const PROGRESS_MESSAGE: Record<Pending, string> = {
  join: "Joining the cluster. This node is contacting the cluster address and copying its data, which can take a minute.",
};

const HEADING: Record<Pending, string> = {
  join: "Joining a cluster",
};

const ADDRESS_EXAMPLE = "Use host:port, for example 192.168.40.58:7000.";

const IPV6_EXAMPLE = "Use [address]:port, for example [2001:db8::1]:7000.";

export const validateSeedAddress = (value: string): string => {
  const address = value.trim();
  if (!address) return "";

  if (address.includes("://")) {
    return `Enter a cluster address, not a URL. ${ADDRESS_EXAMPLE}`;
  }

  let rest: string;

  if (address.startsWith("[")) {
    const close = address.indexOf("]");
    if (close < 0) {
      return `Close the bracket around the IPv6 address. ${IPV6_EXAMPLE}`;
    }

    rest = address.slice(close + 1);
  } else {
    const separator = address.lastIndexOf(":");
    if (separator <= 0) {
      return `The cluster port is missing. ${ADDRESS_EXAMPLE}`;
    }

    if (address.slice(0, separator).includes(":")) {
      return `Wrap an IPv6 address in brackets. ${IPV6_EXAMPLE}`;
    }

    rest = address.slice(separator);
  }

  if (!rest.startsWith(":") || rest.length < 2) {
    return `The cluster port is missing. ${ADDRESS_EXAMPLE}`;
  }

  const port = rest.slice(1);
  if (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535) {
    return "The port must be a number between 1 and 65535.";
  }

  return "";
};

const ClusterSetupCard = ({
  state,
  failure,
  joinOnly = false,
  disabledReason,
  onSubmitted,
  onCancel,
}: Props) => {
  const { showSnackbar } = useSnackbar();
  const [token, setToken] = useState("");
  const [editingToken, setEditingToken] = useState(true);
  const [seedAddress, setSeedAddress] = useState("");
  const [addressTouched, setAddressTouched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);

  const onSubmittedRef = useRef(onSubmitted);
  const notifiedRef = useRef("");

  useEffect(() => {
    onSubmittedRef.current = onSubmitted;
  });

  const notifyFailure = useCallback(
    (message: string) => {
      if (!message || message === notifiedRef.current) return;
      notifiedRef.current = message;
      showSnackbar(message, "error");
    },
    [showSnackbar],
  );

  useEffect(() => {
    if (pending !== null || !failure) return;
    notifyFailure(failure);
  }, [failure, pending, notifyFailure]);

  useEffect(() => {
    if (pending === null) return;

    let cancelled = false;
    const deadline = Date.now() + WATCH_TIMEOUT_MS;

    const giveUp = (reason: string) => {
      if (cancelled) return;
      notifyFailure(reason);
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
  }, [pending, notifyFailure]);

  const blocked = disabledReason !== undefined;
  const working = submitting || pending !== null || state === "joining";
  const busy = blocked || working;

  const addressError = addressTouched ? validateSeedAddress(seedAddress) : "";
  const submittable =
    token.trim() !== "" &&
    seedAddress.trim() !== "" &&
    validateSeedAddress(seedAddress) === "";

  const commitToken = (value: string) => {
    setToken(value);
    if (value.trim() !== "") setEditingToken(false);
  };

  const clearToken = () => {
    setToken("");
    setEditingToken(true);
  };

  const run = async (what: Pending, fn: () => Promise<unknown>) => {
    setSubmitting(true);
    notifiedRef.current = "";

    try {
      await fn();
      setPending(what);
    } catch (err) {
      notifyFailure(err instanceof Error ? err.message : String(err));
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

      {pending === null && (
        <Box sx={{ mt: 1 }}>
          {editingToken ? (
            <TextField
              fullWidth
              label="Join token"
              placeholder="Paste the join token"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              onPaste={(e) => {
                const pasted = e.clipboardData.getData("text").trim();
                if (!pasted) return;
                e.preventDefault();
                commitToken(pasted);
              }}
              onBlur={() => commitToken(token)}
              disabled={busy}
              margin="normal"
            />
          ) : (
            <Box sx={{ mt: 2, mb: 1 }}>
              <Typography variant="caption" color="textSecondary">
                Join token
              </Typography>
              <Stack
                direction="row"
                spacing={1}
                sx={{
                  alignItems: "center",
                  border: "1px solid",
                  borderColor: "divider",
                  borderRadius: 1,
                  px: 1.5,
                  py: 1,
                  mt: 0.5,
                }}
              >
                <CheckCircleIcon color="success" fontSize="small" />
                <Typography variant="body2" sx={{ flexGrow: 1 }}>
                  Token pasted · ends in {token.trim().slice(-4)}
                </Typography>
                <IconButton
                  size="small"
                  aria-label="Clear join token"
                  disabled={busy}
                  onClick={clearToken}
                >
                  <CloseIcon fontSize="small" />
                </IconButton>
              </Stack>
            </Box>
          )}

          <TextField
            fullWidth
            label="Cluster address"
            placeholder="10.0.0.1:7000"
            value={seedAddress}
            onChange={(e) => setSeedAddress(e.target.value)}
            onBlur={() => setAddressTouched(true)}
            error={addressError !== ""}
            helperText={
              addressError ||
              "The cluster address of a node already in the cluster."
            }
            disabled={busy}
            margin="normal"
          />

          <Button
            variant="contained"
            color="success"
            fullWidth
            sx={{ mt: 2 }}
            disabled={busy || !submittable}
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
            onClick={() => onCancel?.()}
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
