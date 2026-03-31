import React, { useState, useRef, useCallback } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Tabs,
  Tab,
  Box,
  Button,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  CircularProgress,
  Alert,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  Chip,
  Typography,
  IconButton,
  Collapse,
} from "@mui/material";
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  Close as CloseIcon,
  DragIndicator as DragIcon,
} from "@mui/icons-material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  listNetworkRules,
  createNetworkRule,
  updateNetworkRule,
  deleteNetworkRule,
  reorderNetworkRule,
  type NetworkRule,
  type ListNetworkRulesResponse,
} from "@/queries/network_rules";
import { useAuth } from "@/contexts/AuthContext";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import * as yup from "yup";
import { ValidationError } from "yup";
import { formatProtocol, PROTOCOL_CHIP_COLORS } from "@/utils/formatters";
import { PROTOCOL_NAMES } from "@/utils/formatters";

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

interface PolicyRulesModalProps {
  open: boolean;
  onClose: () => void;
  policyName: string;
}

type Direction = "uplink" | "downlink";
type Action = "allow" | "deny";

interface FormValues {
  description: string;
  direction: Direction;
  action: Action;
  remotePrefix: string;
  protocol: string;
  portLow: string;
  portHigh: string;
}

const schema = yup.object().shape({
  description: yup.string(),
  direction: yup
    .string()
    .oneOf(["uplink", "downlink"], "Invalid direction")
    .required("Direction is required"),
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
  policyName,
}) => {
  const { accessToken } = useAuth();
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState<Direction>("uplink");
  const [isFormDialogOpen, setFormDialogOpen] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<number | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [selectedRuleId, setSelectedRuleId] = useState<number | null>(null);
  const [formValues, setFormValues] = useState<FormValues>({
    description: "",
    direction: "uplink",
    action: "allow",
    remotePrefix: "",
    protocol: "",
    portLow: "",
    portHigh: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [formAlert, setFormAlert] = useState<string>("");
  const [formLoading, setFormLoading] = useState(false);

  const { data, isLoading } = useQuery<ListNetworkRulesResponse>({
    queryKey: ["networkRules", policyName],
    queryFn: () =>
      accessToken
        ? listNetworkRules(accessToken, policyName)
        : Promise.resolve({ items: [], page: 1, per_page: 50, total_count: 0 }),
    enabled: open && !!accessToken,
  });

  const rules = data?.items || [];
  const filteredRules = rules.filter((rule) => rule.direction === activeTab);

  const dragIndexRef = useRef<number | null>(null);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const [reorderError, setReorderError] = useState<string | null>(null);

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
    (targetIndex: number) => async (e: React.DragEvent) => {
      e.preventDefault();
      setHoverIndex(null);

      const sourceIndex = dragIndexRef.current;
      if (sourceIndex === null || sourceIndex === targetIndex) {
        dragIndexRef.current = null;
        return;
      }

      const originalRules = [...filteredRules];

      try {
        const newRules = [...filteredRules];
        const [moved] = newRules.splice(sourceIndex, 1);
        newRules.splice(targetIndex, 0, moved);

        // Optimistically update the UI
        queryClient.setQueryData<ListNetworkRulesResponse>(
          ["networkRules", policyName],
          {
            ...data!,
            items: rules.map((r: NetworkRule) => {
              const newRule = newRules.find((nr) => nr.id === r.id);
              return newRule || r;
            }),
          },
        );

        const ruleId = originalRules[sourceIndex]?.id;
        if (!ruleId) return;
        await reorderNetworkRule(accessToken!, policyName, ruleId, targetIndex);

        await queryClient.invalidateQueries({
          queryKey: ["networkRules", policyName],
        });
      } catch (err) {
        // Restore original order on error
        queryClient.setQueryData<ListNetworkRulesResponse>(
          ["networkRules", policyName],
          {
            ...data!,
            items: rules,
          },
        );
        const errorMessage =
          err instanceof Error ? err.message : "Failed to reorder rule";
        setReorderError(errorMessage);
      } finally {
        dragIndexRef.current = null;
      }
    },
    [filteredRules, accessToken, policyName, queryClient, data, rules],
  );

  const handleDragEnd = useCallback(() => {
    dragIndexRef.current = null;
    setHoverIndex(null);
  }, []);

  const getEditingRule = (): NetworkRule | undefined => {
    return editingRuleId
      ? rules.find((r: NetworkRule) => r.id === editingRuleId)
      : undefined;
  };

  const resetForm = (direction: Direction = "uplink") => {
    setFormValues({
      description: "",
      direction,
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
    resetForm(activeTab);
    setFormDialogOpen(true);
  };

  const handleEditRule = (rule: NetworkRule) => {
    setFormValues({
      description: rule.description,
      direction: rule.direction,
      action: rule.action,
      remotePrefix: rule.remote_prefix || "",
      protocol: rule.protocol ? String(rule.protocol) : "",
      portLow: rule.port_low ? String(rule.port_low) : "",
      portHigh: rule.port_high ? String(rule.port_high) : "",
    });
    setErrors({});
    setTouched({});
    setFormAlert("");
    setEditingRuleId(rule.id);
    setFormDialogOpen(true);
  };

  const handleDeleteRule = (rule: NetworkRule) => {
    setSelectedRuleId(rule.id);
    setDeleteConfirmOpen(true);
  };

  const handleFormChange = (field: keyof FormValues, value: string) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    validateField(field, value);
  };

  const handleFormBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const handleFormSubmit = async () => {
    if (!accessToken) return;

    const isValid = await validateForm();
    if (!isValid) return;

    setFormLoading(true);
    setFormAlert("");

    try {
      const protocol = formValues.protocol
        ? parseProtocol(formValues.protocol)
        : undefined;
      const portLow = formValues.portLow
        ? Number(formValues.portLow)
        : undefined;
      const portHigh = formValues.portHigh
        ? Number(formValues.portHigh)
        : undefined;

      const directionRules = rules.filter(
        (rule: NetworkRule) => rule.direction === formValues.direction,
      );
      const maxPrecedence =
        directionRules.length > 0
          ? Math.max(...directionRules.map((r: NetworkRule) => r.precedence))
          : 0;
      const precedence = maxPrecedence + 100;

      const remotePrefix = formValues.remotePrefix || undefined;

      if (editingRuleId) {
        await updateNetworkRule(
          accessToken,
          policyName,
          editingRuleId,
          formValues.direction,
          formValues.action,
          precedence,
          remotePrefix,
          protocol,
          portLow,
          portHigh,
        );
      } else {
        await createNetworkRule(
          accessToken,
          policyName,
          formValues.description,
          formValues.direction,
          formValues.action,
          precedence,
          remotePrefix,
          protocol,
          portLow,
          portHigh,
        );
      }

      await queryClient.invalidateQueries({
        queryKey: ["networkRules", policyName],
      });

      setFormDialogOpen(false);
      resetForm();
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred";
      setFormAlert(errorMessage);
    } finally {
      setFormLoading(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!accessToken || selectedRuleId === null) return;

    try {
      // deleteNetworkRule expects (authToken, policyName, ruleId)
      await deleteNetworkRule(accessToken, policyName, selectedRuleId);
      await queryClient.invalidateQueries({
        queryKey: ["networkRules", policyName],
      });
      setDeleteConfirmOpen(false);
      setSelectedRuleId(null);
    } catch (error) {
      console.error("Failed to delete rule:", error);
    }
  };

  const getDirectionColor = (
    direction: "uplink" | "downlink",
  ):
    | "default"
    | "primary"
    | "secondary"
    | "error"
    | "warning"
    | "info"
    | "success" => {
    switch (direction) {
      case "uplink":
        return "info";
      case "downlink":
        return "success";
      default:
        return "default";
    }
  };

  const getActionColor = (
    action: "allow" | "deny",
  ):
    | "default"
    | "primary"
    | "secondary"
    | "error"
    | "warning"
    | "info"
    | "success" => {
    switch (action) {
      case "allow":
        return "success";
      case "deny":
        return "error";
      default:
        return "default";
    }
  };

  const selectedRule = getEditingRule();

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
          Network Rules for {policyName}
        </DialogTitle>

        <DialogContent dividers>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Rules are evaluated in order, top to bottom. The first matching rule
            is applied.
          </Typography>
          <Box sx={{ borderBottom: 1, borderColor: "divider", mb: 2 }}>
            <Tabs
              value={activeTab}
              onChange={(_, newValue) => setActiveTab(newValue)}
              aria-label="rule direction tabs"
            >
              <Tab label="Uplink" value="uplink" />
              <Tab label="Downlink" value="downlink" />
            </Tabs>
          </Box>

          {reorderError && (
            <Alert
              severity="error"
              onClose={() => setReorderError(null)}
              sx={{ mb: 2 }}
            >
              {reorderError}
            </Alert>
          )}

          {isLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", p: 3 }}>
              <CircularProgress />
            </Box>
          ) : filteredRules.length === 0 ? (
            <Typography color="text.secondary" sx={{ p: 2 }}>
              No network rules configured for {activeTab} direction.
            </Typography>
          ) : (
            <List disablePadding>
              {filteredRules.map((rule: NetworkRule, index: number) => (
                <ListItem
                  key={rule.id}
                  draggable
                  onDragStart={handleDragStart(index)}
                  onDragOver={handleDragOver(index)}
                  onDrop={handleDrop(index)}
                  onDragEnd={handleDragEnd}
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "flex-start",
                    py: 1.5,
                    borderBottom: "1px solid",
                    borderColor: "divider",
                    backgroundColor:
                      hoverIndex === index ? "action.hover" : "transparent",
                    cursor: "move",
                    transition: "background-color 0.2s",
                    "&:last-child": {
                      borderBottom: "none",
                    },
                  }}
                >
                  <ListItemIcon sx={{ minWidth: "auto", mr: 1 }}>
                    <DragIcon fontSize="small" />
                  </ListItemIcon>
                  <ListItemText
                    primary={
                      <Box
                        sx={{
                          display: "flex",
                          gap: 1,
                          alignItems: "center",
                          flexWrap: "wrap",
                        }}
                      >
                        <Chip
                          label={rule.action.toUpperCase()}
                          size="small"
                          color={getActionColor(rule.action)}
                          variant="outlined"
                        />
                        <Chip
                          label={
                            rule.direction === "uplink"
                              ? "TO"
                              : rule.direction === "downlink"
                                ? "FROM"
                                : "ANY"
                          }
                          size="small"
                          color={getDirectionColor(rule.direction)}
                          variant="outlined"
                        />
                        {rule.remote_prefix && (
                          <Chip
                            label={rule.remote_prefix}
                            size="small"
                            variant="outlined"
                          />
                        )}
                        <Chip
                          label={
                            rule.protocol === 0
                              ? "any protocol"
                              : formatProtocol(rule.protocol)
                          }
                          size="small"
                          variant="outlined"
                          sx={{
                            borderColor:
                              PROTOCOL_CHIP_COLORS[rule.protocol] || "divider",
                            color:
                              PROTOCOL_CHIP_COLORS[rule.protocol] ||
                              "text.primary",
                          }}
                        />
                        <Chip
                          label={
                            rule.port_low === 0 && rule.port_high === 0
                              ? "any port"
                              : rule.port_low === rule.port_high
                                ? String(rule.port_low)
                                : `${rule.port_low}-${rule.port_high}`
                          }
                          size="small"
                          variant="outlined"
                        />
                      </Box>
                    }
                    primaryTypographyProps={{
                      component: "div",
                    }}
                  />
                  <Box sx={{ display: "flex", gap: 0.5 }}>
                    <IconButton
                      size="small"
                      onClick={() => handleEditRule(rule)}
                      title="Edit rule"
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                    <IconButton
                      size="small"
                      onClick={() => handleDeleteRule(rule)}
                      title="Delete rule"
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Box>
                </ListItem>
              ))}
            </List>
          )}
        </DialogContent>

        <DialogActions>
          <Button onClick={onClose}>Close</Button>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={handleOpenCreateForm}
          >
            Add Rule
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={isFormDialogOpen}
        onClose={() => !formLoading && setFormDialogOpen(false)}
        aria-labelledby="rule-form-dialog-title"
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle id="rule-form-dialog-title">
          {editingRuleId ? "Edit Rule" : "Create Rule"}
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
            disabled={formLoading}
          />

          <FormControl fullWidth margin="normal">
            <InputLabel id="direction-label">Direction</InputLabel>
            <Select
              labelId="direction-label"
              label="Direction"
              value={formValues.direction}
              onChange={(e) => handleFormChange("direction", e.target.value)}
              onBlur={() => handleFormBlur("direction")}
              error={!!errors.direction && touched.direction}
              disabled={formLoading}
            >
              <MenuItem value="uplink">Uplink</MenuItem>
              <MenuItem value="downlink">Downlink</MenuItem>
            </Select>
          </FormControl>

          <FormControl fullWidth margin="normal">
            <InputLabel id="action-label">Action</InputLabel>
            <Select
              labelId="action-label"
              label="Action"
              value={formValues.action}
              onChange={(e) => handleFormChange("action", e.target.value)}
              onBlur={() => handleFormBlur("action")}
              error={!!errors.action && touched.action}
              disabled={formLoading}
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
            disabled={formLoading}
          />

          <TextField
            fullWidth
            label="Protocol"
            placeholder="tcp, udp, icmp, or 0-255"
            value={formValues.protocol}
            onChange={(e) => handleFormChange("protocol", e.target.value)}
            onBlur={() => handleFormBlur("protocol")}
            error={!!errors.protocol && touched.protocol}
            helperText={
              touched.protocol
                ? errors.protocol
                : "Optional (name or number: tcp, udp, 6, 17)"
            }
            margin="normal"
            disabled={formLoading}
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
              disabled={formLoading}
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
              disabled={formLoading}
              sx={{ flex: 1 }}
            />
          </Box>
        </DialogContent>

        <DialogActions>
          <Button
            onClick={() => setFormDialogOpen(false)}
            disabled={formLoading}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="success"
            onClick={handleFormSubmit}
            disabled={formLoading}
          >
            {formLoading ? "Saving..." : editingRuleId ? "Update" : "Create"}
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
