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

const ALL_CIPHERING = ["NEA0", "NEA1", "NEA2"];
const ALL_INTEGRITY = ["NIA0", "NIA1", "NIA2"];

const ALGORITHM_DESCRIPTIONS: Record<string, React.ReactNode> = {
  NEA0: (
    <>
      NEA0 — Null{" "}
      <Box component="span" sx={{ color: "warning.main", fontWeight: 500 }}>
        (no encryption)
      </Box>
    </>
  ),
  NEA1: "NEA1 — SNOW 3G",
  NEA2: "NEA2 — AES",
  NIA0: (
    <>
      NIA0 — Null{" "}
      <Box component="span" sx={{ color: "warning.main", fontWeight: 500 }}>
        (no protection)
      </Box>
    </>
  ),
  NIA1: "NIA1 — SNOW 3G",
  NIA2: "NIA2 — AES",
};

const isNullAlgorithm = (name: string) => name === "NEA0" || name === "NIA0";

const CANONICAL_ORDER: Record<string, number> = {
  NEA2: 0,
  NEA1: 1,
  NEA0: 2,
  NIA2: 0,
  NIA1: 1,
  NIA0: 2,
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

    const ciphering = enabledCiphering.map((a) => a.name);
    const integrity = enabledIntegrity.map((a) => a.name);

    try {
      await updateOperatorNASSecurity(
        accessToken,
        ciphering,
        integrity,
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
                  primary={ALGORITHM_DESCRIPTIONS[entry.name] || entry.name}
                  primaryTypographyProps={{
                    variant: "body2",
                    color: "text.primary",
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
