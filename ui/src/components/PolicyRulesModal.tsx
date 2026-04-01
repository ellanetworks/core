import React, { useState, useRef, useCallback } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Box,
  Button,
  TextField,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Alert,
  Chip,
  Typography,
  IconButton,
  Collapse,
  Autocomplete,
} from "@mui/material";
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  DragIndicator as DragIcon,
} from "@mui/icons-material";
import {
  updatePolicy,
  type APIPolicy,
  type PolicyRule,
} from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import * as yup from "yup";
import { ValidationError } from "yup";
import {
  formatProtocol,
  PROTOCOL_CHIP_COLORS,
  PROTOCOL_NAMES,
} from "@/utils/formatters";

const parseProtocol = (value: string): number | undefined => {
  if (!value || value.trim() === "") return undefined;
  const trimmed = value.trim().toUpperCase();
  if (/^\d+$/.test(trimmed)) {
    const num = parseInt(trimmed, 10);
    return num >= 0 && num <= 255 ? num : undefined;
  }
  const entry = Object.entries(PROTOCOL_NAMES).find(
    ([, name]) => name.toUpperCase() === trimmed,
  );
  return entry ? parseInt(entry[0], 10) : undefined;
};

const getProtocolOptions = (): Array<{ label: string; value: string }> => {
  const options = Object.entries(PROTOCOL_NAMES).map(([num, name]) => ({
    label: `${name} (${num})`,
    value: name,
  }));
  return options.sort((a, b) => a.label.localeCompare(b.label));
};

interface PolicyRulesModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  policy: APIPolicy;
  direction: "uplink" | "downlink";
}

type Action = "allow" | "deny";

interface InMemoryRule {
  tempId: string;
  description: string;
  action: Action;
  remote_prefix?: string;
  protocol: number;
  port_low: number;
  port_high: number;
}

interface FormValues {
  description: string;
  action: Action;
  remotePrefix: string;
  protocol: string;
  portLow: string;
  portHigh: string;
}

const schema = yup.object().shape({
  description: yup.string(),
  action: yup
    .string()
    .oneOf(["allow", "deny"], "Invalid action")
    .required("Action is required"),
  remotePrefix: yup
    .string()
    .matches(
      /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/,
      "Must be valid CIDR format (e.g., 192.168.0.0/24)",
    ),
  protocol: yup
    .string()
    .test(
      "valid-protocol",
      "Protocol must be a valid name (tcp, udp, icmp) or number 0-255",
      (val) => {
        if (!val) return true;
        return parseProtocol(val) !== undefined;
      },
    ),
  portLow: yup
    .string()
    .test("valid-port-low", "Port Low must be between 0 and 65535", (val) => {
      if (!val) return true;
      const num = Number(val);
      return !isNaN(num) && num >= 0 && num <= 65535;
    }),
  portHigh: yup
    .string()
    .test("valid-port-high", "Port High must be between 0 and 65535", (val) => {
      if (!val) return true;
      const num = Number(val);
      return !isNaN(num) && num >= 0 && num <= 65535;
    }),
});

const PolicyRulesModal: React.FC<PolicyRulesModalProps> = ({
  open,
  onClose,
  onSuccess,
  policy,
  direction,
}) => {
  const { accessToken } = useAuth();

  const directionRules =
    (direction === "uplink" ? policy.rules?.uplink : policy.rules?.downlink) ??
    [];

  const [rules, setRules] = useState<InMemoryRule[]>(() =>
    directionRules.map((rule, idx) => ({
      tempId: `rule-${idx}-${Date.now()}`,
      description: rule.description,
      action: rule.action,
      remote_prefix: rule.remote_prefix,
      protocol: rule.protocol,
      port_low: rule.port_low,
      port_high: rule.port_high,
    })),
  );

  const [isFormDialogOpen, setFormDialogOpen] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [selectedRuleId, setSelectedRuleId] = useState<string | null>(null);
  const [formValues, setFormValues] = useState<FormValues>({
    description: "",
    action: "allow",
    remotePrefix: "",
    protocol: "",
    portLow: "",
    portHigh: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [formAlert, setFormAlert] = useState<string>("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const dragIndexRef = useRef<number | null>(null);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const handleDragStart = useCallback(
    (index: number) => (e: React.DragEvent) => {
      dragIndexRef.current = index;
      e.dataTransfer.effectAllowed = "move";
      const el = e.currentTarget as HTMLElement;
      e.dataTransfer.setDragImage(el, el.offsetWidth / 2, el.offsetHeight / 2);
    },
    [],
  );

  const handleDragOver = useCallback(
    (index: number) => (e: React.DragEvent) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
      setHoverIndex(index);
    },
    [],
  );

  const handleDrop = useCallback(
    (targetIndex: number) => (e: React.DragEvent) => {
      e.preventDefault();
      setHoverIndex(null);

      const sourceIndex = dragIndexRef.current;
      if (sourceIndex === null || sourceIndex === targetIndex) {
        dragIndexRef.current = null;
        return;
      }

      setRules((prev) => {
        const newRules = [...prev];
        const [moved] = newRules.splice(sourceIndex, 1);
        newRules.splice(targetIndex, 0, moved);
        return newRules;
      });

      dragIndexRef.current = null;
    },
    [],
  );

  const handleDragEnd = useCallback(() => {
    dragIndexRef.current = null;
    setHoverIndex(null);
  }, []);

  const resetForm = () => {
    setFormValues({
      description: "",
      action: "allow",
      remotePrefix: "",
      protocol: "",
      portLow: "",
      portHigh: "",
    });
    setErrors({});
    setTouched({});
    setFormAlert("");
    setEditingRuleId(null);
  };

  const validateField = async (field: string, value: string) => {
    try {
      await schema.validateAt(field, { ...formValues, [field]: value });
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    } catch (err) {
      if (err instanceof ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const validateForm = async (): Promise<boolean> => {
    try {
      await schema.validate(formValues, { abortEarly: false });
      setErrors({});
      return true;
    } catch (err) {
      if (err instanceof ValidationError) {
        const validationErrors = err.inner.reduce(
          (acc, curr) => {
            acc[curr.path!] = curr.message;
            return acc;
          },
          {} as Record<string, string>,
        );
        setErrors(validationErrors);
      }
      return false;
    }
  };

  const handleOpenCreateForm = () => {
    resetForm();
    setFormDialogOpen(true);
  };

  const handleEditRule = (rule: InMemoryRule) => {
    setFormValues({
      description: rule.description,
      action: rule.action,
      remotePrefix: rule.remote_prefix || "",
      protocol: rule.protocol
        ? (PROTOCOL_NAMES[rule.protocol] ?? String(rule.protocol))
        : "",
      portLow: rule.port_low ? String(rule.port_low) : "",
      portHigh: rule.port_high ? String(rule.port_high) : "",
    });
    setErrors({});
    setTouched({});
    setFormAlert("");
    setEditingRuleId(rule.tempId);
    setFormDialogOpen(true);
  };

  const handleDeleteRule = (rule: InMemoryRule) => {
    setSelectedRuleId(rule.tempId);
    setDeleteConfirmOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (selectedRuleId === null) return;
    setRules((prev) => prev.filter((r) => r.tempId !== selectedRuleId));
    setDeleteConfirmOpen(false);
    setSelectedRuleId(null);
  };

  const handleFormChange = (field: keyof FormValues, value: string) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    validateField(field, value);
  };

  const handleFormBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const handleFormSubmit = async () => {
    const isValid = await validateForm();
    if (!isValid) return;

    setFormAlert("");

    const protocol = formValues.protocol
      ? parseProtocol(formValues.protocol)
      : undefined;
    const portLow = formValues.portLow ? Number(formValues.portLow) : undefined;
    const portHigh = formValues.portHigh
      ? Number(formValues.portHigh)
      : undefined;
    const remotePrefix = formValues.remotePrefix || undefined;

    if (editingRuleId) {
      setRules((prev) =>
        prev.map((r) => {
          if (r.tempId === editingRuleId) {
            return {
              ...r,
              description: formValues.description,
              action: formValues.action,
              remote_prefix: remotePrefix,
              protocol: protocol ?? 0,
              port_low: portLow ?? 0,
              port_high: portHigh ?? 0,
            };
          }
          return r;
        }),
      );
    } else {
      const newRule: InMemoryRule = {
        tempId: `temp-${Date.now()}`,
        description: formValues.description,
        action: formValues.action,
        remote_prefix: remotePrefix,
        protocol: protocol ?? 0,
        port_low: portLow ?? 0,
        port_high: portHigh ?? 0,
      };
      setRules((prev) => [...prev, newRule]);
    }

    setFormDialogOpen(false);
    resetForm();
  };

  const handleSave = async () => {
    if (!accessToken) return;
    setSaving(true);
    setSaveError(null);

    try {
      const toApiRules = (arr: InMemoryRule[]): PolicyRule[] =>
        arr.map((r) => ({
          description: r.description,
          remote_prefix: r.remote_prefix,
          protocol: r.protocol,
          port_low: r.port_low,
          port_high: r.port_high,
          action: r.action,
        }));

      const otherDirection =
        direction === "uplink" ? policy.rules?.downlink : policy.rules?.uplink;

      const updatedRules = {
        ...(direction === "uplink"
          ? {
              uplink: toApiRules(rules),
              ...(otherDirection && otherDirection.length > 0
                ? { downlink: otherDirection }
                : {}),
            }
          : {
              downlink: toApiRules(rules),
              ...(otherDirection && otherDirection.length > 0
                ? { uplink: otherDirection }
                : {}),
            }),
      };

      await updatePolicy(
        accessToken,
        policy.name,
        policy.bitrate_uplink,
        policy.bitrate_downlink,
        policy.var5qi,
        policy.arp,
        policy.data_network_name,
        Object.keys(updatedRules).length > 0 ? updatedRules : undefined,
      );

      onClose();
      onSuccess();
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred";
      setSaveError(`Failed to save rules: ${errorMessage}`);
    } finally {
      setSaving(false);
    }
  };

  const formatPorts = (rule: InMemoryRule): string => {
    if (rule.port_low === 0 && rule.port_high === 0) return "any";
    if (rule.port_low === rule.port_high) return String(rule.port_low);
    return `${rule.port_low}-${rule.port_high}`;
  };

  const directionLabel = direction === "uplink" ? "Uplink" : "Downlink";

  return (
    <>
      <Dialog
        open={open}
        onClose={onClose}
        aria-labelledby="policy-rules-modal-title"
        fullWidth
        maxWidth="md"
      >
        <DialogTitle id="policy-rules-modal-title" sx={{ pb: 0 }}>
          Edit {directionLabel} Rules
        </DialogTitle>

        <DialogContent dividers>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Rules are evaluated in order, top to bottom. The first matching rule
            is applied. Drag rows to reorder.
          </Typography>

          {saveError && (
            <Alert
              severity="error"
              onClose={() => setSaveError(null)}
              sx={{ mb: 2 }}
            >
              {saveError}
            </Alert>
          )}

          {rules.length === 0 ? (
            <Typography color="text.secondary" sx={{ p: 2 }}>
              No {direction} rules configured.
            </Typography>
          ) : (
            <Box
              sx={{
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
              }}
            >
              {rules.map((rule, index) => (
                <Box
                  key={rule.tempId}
                  draggable
                  onDragStart={handleDragStart(index)}
                  onDragOver={handleDragOver(index)}
                  onDrop={handleDrop(index)}
                  onDragEnd={handleDragEnd}
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    py: 1,
                    px: 1.5,
                    backgroundColor:
                      hoverIndex === index ? "action.hover" : "transparent",
                    cursor: "grab",
                    transition: "background-color 0.2s",
                    "&:not(:last-child)": {
                      borderBottom: "1px solid",
                      borderColor: "divider",
                    },
                  }}
                >
                  <DragIcon
                    fontSize="small"
                    sx={{ color: "text.secondary", flexShrink: 0, mr: 1 }}
                  />
                  <Typography
                    variant="body2"
                    sx={{
                      fontWeight: 600,
                      width: 24,
                      flexShrink: 0,
                    }}
                  >
                    {index + 1}
                  </Typography>
                  <Box sx={{ width: 72, flexShrink: 0 }}>
                    <Chip
                      label={rule.action.toUpperCase()}
                      size="small"
                      color={rule.action === "allow" ? "success" : "error"}
                      variant="outlined"
                    />
                  </Box>
                  <Box sx={{ width: 100, flexShrink: 0 }}>
                    <Chip
                      label={
                        rule.protocol === 0
                          ? "any"
                          : formatProtocol(rule.protocol)
                      }
                      size="small"
                      variant="outlined"
                      sx={{
                        borderColor:
                          PROTOCOL_CHIP_COLORS[rule.protocol] || "divider",
                        color:
                          PROTOCOL_CHIP_COLORS[rule.protocol] || "text.primary",
                      }}
                    />
                  </Box>
                  <Typography
                    variant="body2"
                    sx={{
                      fontFamily: "monospace",
                      width: 140,
                      flexShrink: 0,
                    }}
                  >
                    {rule.remote_prefix || "any"}
                  </Typography>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ width: 90, flexShrink: 0 }}
                  >
                    {formatPorts(rule)}
                  </Typography>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{
                      flex: 1,
                      minWidth: 0,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {rule.description || "—"}
                  </Typography>
                  <Box sx={{ display: "flex", gap: 0.5, flexShrink: 0, ml: 1 }}>
                    <IconButton
                      size="small"
                      color="primary"
                      onClick={() => handleEditRule(rule)}
                      title="Edit rule"
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                    <IconButton
                      size="small"
                      color="primary"
                      onClick={() => handleDeleteRule(rule)}
                      title="Delete rule"
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Box>
                </Box>
              ))}
            </Box>
          )}

          <Box sx={{ mt: 2 }}>
            <Button
              variant="outlined"
              startIcon={<AddIcon />}
              onClick={handleOpenCreateForm}
              disabled={saving}
              size="small"
            >
              Add Rule
            </Button>
          </Box>
        </DialogContent>

        <DialogActions>
          <Button onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button
            variant="contained"
            color="success"
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? "Saving..." : "Save"}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Rule Form Dialog */}
      <Dialog
        open={isFormDialogOpen}
        onClose={() => setFormDialogOpen(false)}
        aria-labelledby="rule-form-dialog-title"
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle id="rule-form-dialog-title">
          {editingRuleId ? "Edit Rule" : "Add Rule"}
        </DialogTitle>

        <DialogContent dividers>
          <Collapse in={!!formAlert}>
            <Alert
              onClose={() => setFormAlert("")}
              severity="error"
              sx={{ mb: 2 }}
            >
              {formAlert}
            </Alert>
          </Collapse>

          <TextField
            fullWidth
            label="Description"
            value={formValues.description}
            onChange={(e) => handleFormChange("description", e.target.value)}
            onBlur={() => handleFormBlur("description")}
            error={!!errors.description && touched.description}
            helperText={touched.description ? errors.description : ""}
            margin="normal"
            autoFocus
          />

          <FormControl fullWidth margin="normal">
            <InputLabel id="action-label">Action</InputLabel>
            <Select
              labelId="action-label"
              label="Action"
              value={formValues.action}
              onChange={(e) => handleFormChange("action", e.target.value)}
              onBlur={() => handleFormBlur("action")}
              error={!!errors.action && touched.action}
            >
              <MenuItem value="allow">Allow</MenuItem>
              <MenuItem value="deny">Deny</MenuItem>
            </Select>
          </FormControl>

          <TextField
            fullWidth
            label="Remote Prefix (CIDR)"
            placeholder="e.g., 192.168.0.0/24"
            value={formValues.remotePrefix}
            onChange={(e) => handleFormChange("remotePrefix", e.target.value)}
            onBlur={() => handleFormBlur("remotePrefix")}
            error={!!errors.remotePrefix && touched.remotePrefix}
            helperText={touched.remotePrefix ? errors.remotePrefix : "Optional"}
            margin="normal"
          />

          <Autocomplete
            fullWidth
            options={getProtocolOptions()}
            getOptionLabel={(option) => option.label}
            value={
              getProtocolOptions().find(
                (opt) => opt.value === formValues.protocol,
              ) ?? null
            }
            onChange={(_, value) => {
              handleFormChange("protocol", value?.value ?? "");
            }}
            onBlur={() => handleFormBlur("protocol")}
            isOptionEqualToValue={(option, value) =>
              option.value === value.value
            }
            renderInput={(params) => (
              <TextField
                {...params}
                label="Protocol"
                placeholder="Search protocols..."
                error={!!errors.protocol && touched.protocol}
                helperText={
                  touched.protocol
                    ? errors.protocol
                    : "Optional \u2013 search or leave empty for any"
                }
                margin="normal"
              />
            )}
          />

          <Box sx={{ display: "flex", gap: 2 }}>
            <TextField
              label="Port Low"
              type="number"
              placeholder="0-65535"
              value={formValues.portLow}
              onChange={(e) => handleFormChange("portLow", e.target.value)}
              onBlur={() => handleFormBlur("portLow")}
              error={!!errors.portLow && touched.portLow}
              helperText={touched.portLow ? errors.portLow : "Optional"}
              margin="normal"
              sx={{ flex: 1 }}
            />
            <TextField
              label="Port High"
              type="number"
              placeholder="0-65535"
              value={formValues.portHigh}
              onChange={(e) => handleFormChange("portHigh", e.target.value)}
              onBlur={() => handleFormBlur("portHigh")}
              error={!!errors.portHigh && touched.portHigh}
              helperText={touched.portHigh ? errors.portHigh : "Optional"}
              margin="normal"
              sx={{ flex: 1 }}
            />
          </Box>
        </DialogContent>

        <DialogActions>
          <Button onClick={() => setFormDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            color="success"
            onClick={handleFormSubmit}
          >
            {editingRuleId ? "Update" : "Add"}
          </Button>
        </DialogActions>
      </Dialog>

      <DeleteConfirmationModal
        open={deleteConfirmOpen}
        onClose={() => setDeleteConfirmOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Delete Network Rule"
        description="Are you sure you want to delete this network rule? This action cannot be undone."
      />
    </>
  );
};

export default PolicyRulesModal;
