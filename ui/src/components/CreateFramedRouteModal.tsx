// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Autocomplete,
  Button,
  Alert,
  Collapse,
  Box,
  Typography,
  IconButton,
} from "@mui/material";
import { Add as AddIcon, Delete as DeleteIcon } from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import {
  createFramedRoute,
  updateFramedRoute,
  listEligibleSubscribers,
} from "@/queries/data_networks";
import { isValidCidr } from "@/utils/ip";

const MAX_PER_FAMILY = 8;

interface FramedRouteEdit {
  imsi: string;
  ipv4: string[];
  ipv6: string[];
}

interface CreateFramedRouteModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  dataNetwork: string;
  edit?: FramedRouteEdit;
}

const CreateFramedRouteModal: React.FC<CreateFramedRouteModalProps> = ({
  open,
  onClose,
  onSuccess,
  dataNetwork,
  edit,
}) => {
  const { accessToken, authReady } = useAuth();
  const isEdit = !!edit;

  const [imsi, setImsi] = useState<string>(edit?.imsi ?? "");
  const [ipv4, setIpv4] = useState<string[]>(edit?.ipv4 ?? []);
  const [ipv6, setIpv6] = useState<string[]>(edit?.ipv6 ?? []);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");

  useEffect(() => {
    if (open) {
      setImsi(edit?.imsi ?? "");
      setIpv4(edit?.ipv4 ?? []);
      setIpv6(edit?.ipv6 ?? []);
      setAlert("");
    }
  }, [open, edit]);

  const { data: subscribers } = useQuery({
    queryKey: ["eligible-subscribers", dataNetwork],
    queryFn: () => listEligibleSubscribers(accessToken!, dataNetwork),
    enabled: open && !isEdit && authReady && !!accessToken,
  });

  const cleanV4 = ipv4.map((p) => p.trim()).filter((p) => p !== "");
  const cleanV6 = ipv6.map((p) => p.trim()).filter((p) => p !== "");
  const hasInvalid = [...ipv4, ...ipv6].some(
    (p) => p.trim() !== "" && !isValidCidr(p.trim()),
  );
  const overCap =
    cleanV4.length > MAX_PER_FAMILY || cleanV6.length > MAX_PER_FAMILY;

  const canSubmit =
    imsi.trim() !== "" &&
    cleanV4.length + cleanV6.length > 0 &&
    !hasInvalid &&
    !overCap &&
    !loading;

  const handleSubmit = async () => {
    if (!accessToken) return;

    setLoading(true);
    setAlert("");

    try {
      if (isEdit && edit) {
        await updateFramedRoute(
          accessToken,
          dataNetwork,
          edit.imsi,
          cleanV4,
          cleanV6,
        );
      } else {
        await createFramedRoute(
          accessToken,
          dataNetwork,
          imsi.trim(),
          cleanV4,
          cleanV6,
        );
      }

      onClose();
      onSuccess();
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert(message);
    } finally {
      setLoading(false);
    }
  };

  const renderPrefixRows = (
    title: string,
    addLabel: string,
    placeholder: string,
    values: string[],
    setValues: React.Dispatch<React.SetStateAction<string[]>>,
  ) => (
    <Box sx={{ mt: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        {title}
      </Typography>
      {values.map((value, index) => {
        const invalid = value.trim() !== "" && !isValidCidr(value.trim());
        return (
          <Box
            key={index}
            sx={{ display: "flex", alignItems: "flex-start", gap: 1, mb: 1 }}
          >
            <TextField
              fullWidth
              size="small"
              value={value}
              onChange={(e) =>
                setValues((prev) =>
                  prev.map((p, i) => (i === index ? e.target.value : p)),
                )
              }
              placeholder={placeholder}
              error={invalid}
              helperText={invalid ? "Enter a valid CIDR prefix." : ""}
            />
            <IconButton
              aria-label="Remove prefix"
              onClick={() =>
                setValues((prev) => prev.filter((_, i) => i !== index))
              }
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Box>
        );
      })}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={() => setValues((prev) => [...prev, ""])}
        disabled={values.length >= MAX_PER_FAMILY}
      >
        {addLabel}
      </Button>
    </Box>
  );

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>
        {isEdit ? "Edit Framed Routes" : "Add Framed Routes"}
      </DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert onClose={() => setAlert("")} sx={{ mb: 2 }} severity="error">
            {alert}
          </Alert>
        </Collapse>
        {isEdit ? (
          <TextField
            fullWidth
            label="Subscriber"
            value={imsi}
            margin="normal"
            disabled
          />
        ) : (
          <Autocomplete
            options={(subscribers ?? []).map((s) => s.imsi)}
            value={imsi || null}
            onChange={(_, value) => setImsi(value ?? "")}
            renderInput={(params) => (
              <TextField
                {...params}
                label="Subscriber"
                margin="normal"
                autoFocus
              />
            )}
          />
        )}
        {renderPrefixRows(
          "IPv4 prefixes",
          "Add IPv4 prefix",
          "e.g., 192.168.60.0/24",
          ipv4,
          setIpv4,
        )}
        {renderPrefixRows(
          "IPv6 prefixes",
          "Add IPv6 prefix",
          "e.g., fd00:60::/64",
          ipv6,
          setIpv6,
        )}
        <Typography variant="caption" color="textSecondary" sx={{ mt: 2, display: "block" }}>
          Up to {MAX_PER_FAMILY} prefixes per family. NAT must be disabled to use
          framed routes.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!canSubmit}
        >
          {loading ? "Saving..." : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateFramedRouteModal;
