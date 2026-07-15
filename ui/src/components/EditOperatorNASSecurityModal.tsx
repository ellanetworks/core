// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState, useEffect, useRef, useCallback } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
  Alert,
  Collapse,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  Checkbox,
  Typography,
  Box,
  Divider,
} from "@mui/material";
import { DragIndicator as DragIcon } from "@mui/icons-material";
import { updateOperatorNASSecurity } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorNASSecurityModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    ciphering: string[];
    integrity: string[];
  };
}

interface AlgorithmEntry {
  name: string;
  enabled: boolean;
}

// NAS security algorithms use the RAT-neutral identities shared by 4G (EEA/EIA)
// and 5G (NEA/NIA): NULL, SNOW3G, AES.
const ALL_CIPHERING = ["NULL", "SNOW3G", "AES"];
const ALL_INTEGRITY = ["NULL", "SNOW3G", "AES"];

// describeAlgorithm renders an algorithm with its 3GPP identifiers for the given
// function (ciphering or integrity) across both RATs: 4G EEA/EIA (TS 24.301
// §9.9.3.23) and 5G NEA/NIA (TS 24.501 §9.11.3.34).
const describeAlgorithm = (name: string, kind: string): React.ReactNode => {
  const cipher = kind === "ciphering";
  const fourG = { NULL: "EEA0", SNOW3G: "128-EEA1", AES: "128-EEA2" }[name];
  const fiveG = { NULL: "NEA0", SNOW3G: "128-NEA1", AES: "128-NEA2" }[name];
  const ids =
    fourG && fiveG
      ? ` — ${cipher ? fourG : fourG.replace("EEA", "EIA")} (4G) / ${
          cipher ? fiveG : fiveG.replace("NEA", "NIA")
        } (5G)`
      : "";

  if (name === "NULL") {
    return (
      <>
        NULL{ids}{" "}
        <Box component="span" sx={{ color: "warning.main", fontWeight: 500 }}>
          (no {cipher ? "encryption" : "integrity"})
        </Box>
      </>
    );
  }

  return `${name === "SNOW3G" ? "SNOW 3G" : name}${ids}`;
};

const isNullAlgorithm = (name: string) => name === "NULL";

const CANONICAL_ORDER: Record<string, number> = {
  AES: 0,
  SNOW3G: 1,
  NULL: 2,
};

const buildEntries = (enabled: string[], all: string[]): AlgorithmEntry[] => {
  const entries: AlgorithmEntry[] = enabled.map((name) => ({
    name,
    enabled: true,
  }));
  const disabled = all
    .filter((name) => !enabled.includes(name))
    .sort((a, b) => (CANONICAL_ORDER[a] ?? 0) - (CANONICAL_ORDER[b] ?? 0));
  for (const name of disabled) {
    entries.push({ name, enabled: false });
  }
  return entries;
};

const EditOperatorNASSecurityModal: React.FC<
  EditOperatorNASSecurityModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [ciphering, setCiphering] = useState<AlgorithmEntry[]>([]);
  const [integrity, setIntegrity] = useState<AlgorithmEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setCiphering(buildEntries(initialData.ciphering, ALL_CIPHERING));
      setIntegrity(buildEntries(initialData.integrity, ALL_INTEGRITY));
      setAlert({ message: "" });
    }
  }, [open, initialData]);

  const enabledCiphering = ciphering.filter((a) => a.enabled);
  const enabledIntegrity = integrity.filter((a) => a.enabled);

  const isValid = enabledCiphering.length > 0 && enabledIntegrity.length > 0;

  const nullCipheringPreferred = isNullAlgorithm(enabledCiphering[0]?.name);
  const nullIntegrityPreferred = isNullAlgorithm(enabledIntegrity[0]?.name);
  const nullCipheringOffered = enabledCiphering.some((a) =>
    isNullAlgorithm(a.name),
  );
  const nullIntegrityOffered = enabledIntegrity.some((a) =>
    isNullAlgorithm(a.name),
  );

  const handleToggle =
    (
      list: AlgorithmEntry[],
      setList: React.Dispatch<React.SetStateAction<AlgorithmEntry[]>>,
    ) =>
    (index: number) => {
      const newList = [...list];
      newList[index] = { ...newList[index], enabled: !newList[index].enabled };
      setList(newList);
    };

  const handleSubmit = async () => {
    if (!accessToken) return;
    if (!isValid) return;

    setLoading(true);
    setAlert({ message: "" });

    const cipheringNames = enabledCiphering.map((a) => a.name);
    const integrityNames = enabledIntegrity.map((a) => a.name);

    try {
      await updateOperatorNASSecurity(
        accessToken,
        cipheringNames,
        integrityNames,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update security algorithms: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  // --- Native HTML5 drag-and-drop logic ---
  const dragIndexRef = useRef<number | null>(null);
  const dragListIdRef = useRef<string | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);
  const [dragOverListId, setDragOverListId] = useState<string | null>(null);

  const handleDragStart = useCallback(
    (listId: string, index: number) => (e: React.DragEvent) => {
      dragIndexRef.current = index;
      dragListIdRef.current = listId;
      e.dataTransfer.effectAllowed = "move";
      // Transparent drag image — the visual feedback comes from the highlight
      const el = e.currentTarget as HTMLElement;
      e.dataTransfer.setDragImage(el, el.offsetWidth / 2, el.offsetHeight / 2);
    },
    [],
  );

  const handleDragOver = useCallback(
    (listId: string, index: number) => (e: React.DragEvent) => {
      e.preventDefault();
      if (dragListIdRef.current !== listId) return;
      e.dataTransfer.dropEffect = "move";
      setDragOverIndex(index);
      setDragOverListId(listId);
    },
    [],
  );

  const handleDrop = useCallback(
    (
      listId: string,
      index: number,
      list: AlgorithmEntry[],
      setList: React.Dispatch<React.SetStateAction<AlgorithmEntry[]>>,
    ) =>
      (e: React.DragEvent) => {
        e.preventDefault();
        if (dragListIdRef.current !== listId || dragIndexRef.current === null)
          return;
        const fromIndex = dragIndexRef.current;
        if (fromIndex === index) return;
        const newList = [...list];
        const [moved] = newList.splice(fromIndex, 1);
        newList.splice(index, 0, moved);
        setList(newList);
        dragIndexRef.current = null;
        dragListIdRef.current = null;
        setDragOverIndex(null);
        setDragOverListId(null);
      },
    [],
  );

  const handleDragEnd = useCallback(() => {
    dragIndexRef.current = null;
    dragListIdRef.current = null;
    setDragOverIndex(null);
    setDragOverListId(null);
  }, []);

  const renderAlgorithmList = (
    listId: string,
    title: string,
    list: AlgorithmEntry[],
    setList: React.Dispatch<React.SetStateAction<AlgorithmEntry[]>>,
  ) => {
    const toggle = handleToggle(list, setList);

    return (
      <Box sx={{ mb: 2 }}>
        <Typography variant="subtitle2" sx={{ mb: 1 }}>
          {title}
        </Typography>
        <List dense disablePadding>
          {list.map((entry, index) => {
            const isDragTarget =
              dragOverListId === listId && dragOverIndex === index;
            return (
              <ListItem
                key={entry.name}
                draggable
                onDragStart={handleDragStart(listId, index)}
                onDragOver={handleDragOver(listId, index)}
                onDrop={handleDrop(listId, index, list, setList)}
                onDragEnd={handleDragEnd}
                disablePadding
                sx={{
                  pl: 0.5,
                  pr: 1,
                  opacity: entry.enabled ? 1 : 0.5,
                  borderTop: isDragTarget
                    ? "2px solid"
                    : "2px solid transparent",
                  borderColor: isDragTarget ? "primary.main" : "transparent",
                  transition: "border-color 0.15s ease",
                  cursor: "grab",
                  "&:active": { cursor: "grabbing" },
                  userSelect: "none",
                }}
              >
                <ListItemIcon sx={{ minWidth: 28, color: "text.disabled" }}>
                  <DragIcon fontSize="small" />
                </ListItemIcon>
                <ListItemIcon sx={{ minWidth: 36 }}>
                  <Checkbox
                    edge="start"
                    checked={entry.enabled}
                    onChange={() => toggle(index)}
                    size="small"
                  />
                </ListItemIcon>
                <ListItemText
                  primary={describeAlgorithm(entry.name, listId)}
                  slotProps={{
                    primary: {
                      variant: "body2",
                      color: "textPrimary",
                    },
                  }}
                />
              </ListItem>
            );
          })}
        </List>
        {list.filter((a) => a.enabled).length === 0 && (
          <Alert severity="error" sx={{ mt: 1 }}>
            At least one algorithm must be enabled.
          </Alert>
        )}
      </Box>
    );
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-security-modal-title"
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle id="edit-operator-security-modal-title">
        Edit NAS Security Algorithms
      </DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="error"
          >
            {alert.message}
          </Alert>
        </Collapse>
        <DialogContentText sx={{ mb: 2 }}>
          Configure the security algorithms used to protect NAS signaling
          between the subscriber and Ella Core. The order determines which
          algorithm Ella Core prefers.
        </DialogContentText>

        {(nullCipheringPreferred || nullIntegrityPreferred) && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            NULL is the preferred{" "}
            {nullCipheringPreferred && nullIntegrityPreferred
              ? "ciphering and integrity algorithm"
              : nullCipheringPreferred
                ? "ciphering algorithm"
                : "integrity algorithm"}
            , so NAS signaling will normally carry no{" "}
            {nullCipheringPreferred && nullIntegrityPreferred
              ? "encryption or integrity protection"
              : nullCipheringPreferred
                ? "encryption"
                : "integrity protection"}
            . Move another algorithm above NULL to prefer it.
          </Alert>
        )}

        {!nullCipheringPreferred &&
          !nullIntegrityPreferred &&
          (nullCipheringOffered || nullIntegrityOffered) && (
            <Alert severity="warning" sx={{ mb: 2 }}>
              NULL is enabled as a fallback. Subscribers that offer no stronger
              algorithm will use unprotected NAS signaling.
            </Alert>
          )}

        {renderAlgorithmList(
          "ciphering",
          "Ciphering Preference",
          ciphering,
          setCiphering,
        )}
        <Divider sx={{ my: 1 }} />
        {renderAlgorithmList(
          "integrity",
          "Integrity Preference",
          integrity,
          setIntegrity,
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!isValid || loading}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditOperatorNASSecurityModal;
