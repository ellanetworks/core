import React, { useCallback, useState, useEffect, useRef } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  FormControl,
  Select,
  InputLabel,
  Button,
  MenuItem,
  Alert,
  Collapse,
  Tabs,
  Tab,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  Chip,
  Typography,
  IconButton,
  CircularProgress,
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
  getPolicy,
  APIPolicy,
  type PolicyRules,
} from "@/queries/policies";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
} from "@/queries/data_networks";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import * as yup from "yup";
import { ValidationError } from "yup";
import { formatProtocol, PROTOCOL_CHIP_COLORS } from "@/utils/formatters";
import { PROTOCOL_NAMES } from "@/utils/formatters";

interface EditPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIPolicy;
}

type FormState = Omit<APIPolicy, "bitrate_uplink" | "bitrate_downlink"> & {
  bitrateUpValue: number;
  bitrateUpUnit: "Mbps" | "Gbps";
  bitrateDownValue: number;
  bitrateDownUnit: "Mbps" | "Gbps";
};

type Direction = "uplink" | "downlink";
type Action = "allow" | "deny";

interface RuleFormValues {
  description: string;
  direction: Direction;
  action: Action;
  remotePrefix: string;
  protocol: string;
  portLow: string;
  portHigh: string;
}

// In-memory rule structure for editing
interface InMemoryRule {
  id?: number; // Optional, only for existing rules from server
  tempId?: string; // For newly created rules
  description: string;
  direction: Direction;
  action: Action;
  remote_prefix?: string;
  protocol: number;
  port_low: number;
  port_high: number;
  precedence: number;
  created_at: string;
  updated_at: string;
}

const PER_PAGE = 12;

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

const NON_GBR_5QI_OPTIONS: { value: number; label: string }[] = [
  { value: 5, label: "5 — IMS Signalling" },
  { value: 6, label: "6 — TCP (buffered streaming, web)" },
  { value: 7, label: "7 — Voice, live video, gaming" },
  { value: 8, label: "8 — TCP (buffered streaming)" },
  { value: 9, label: "9 — TCP (default)" },
  { value: 69, label: "69 — Mission critical signalling" },
  { value: 70, label: "70 — Mission critical data" },
  { value: 79, label: "79 — V2X messages" },
  { value: 80, label: "80 — Low latency eMBB" },
];

const NON_GBR_5QI_VALUES = NON_GBR_5QI_OPTIONS.map((o) => o.value);

const policySchema = yup.object().shape({
  bitrateUpValue: yup
    .number()
    .min(1, "Bitrate value must be between 1 and 999")
    .max(999, "Bitrate value must be between 1 and 999")
    .required("Bitrate value is required"),
  bitrateUpUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
  bitrateDownValue: yup
    .number()
    .min(1, "Bitrate value must be between 1 and 999")
    .max(999, "Bitrate value must be between 1 and 999")
    .required("Bitrate value is required"),
  bitrateDownUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
  var5qi: yup
    .number()
    .oneOf(
      NON_GBR_5QI_VALUES,
      `5QI must be one of: ${NON_GBR_5QI_VALUES.join(", ")}`,
    )
    .required("5QI is required"),
  arp: yup.number().min(1).max(15).required("ARP is required"),
  data_network_name: yup.string().required("Data Network Name is required."),
});

const ruleSchema = yup.object().shape({
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

const EditPolicyModal: React.FC<EditPolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (open && authReady && !accessToken) {
      navigate("/login");
    }
  }, [open, authReady, accessToken, navigate]);

  // Policy form state
  const [formValues, setFormValues] = useState<FormState>({
    name: "",
    bitrateUpValue: 0,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 0,
    bitrateDownUnit: "Mbps",
    var5qi: 0,
    arp: 0,
    data_network_name: "",
  });

  const [dataNetworks, setDataNetworks] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  // In-memory rules state
  const [activeTab, setActiveTab] = useState<Direction>("uplink");
  const [inMemoryRules, setInMemoryRules] = useState<InMemoryRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [isFormDialogOpen, setFormDialogOpen] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<string | number | null>(
    null,
  );
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [selectedRuleId, setSelectedRuleId] = useState<string | number | null>(
    null,
  );
  const [ruleFormValues, setRuleFormValues] = useState<RuleFormValues>({
    description: "",
    direction: "uplink",
    action: "allow",
    remotePrefix: "",
    protocol: "",
    portLow: "",
    portHigh: "",
  });
  const [ruleErrors, setRuleErrors] = useState<Record<string, string>>({});
  const [ruleTouched, setRuleTouched] = useState<Record<string, boolean>>({});
  const [ruleFormAlert, setRuleFormAlert] = useState<string>("");

  const dragIndexRef = useRef<number | null>(null);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  // Fetch full policy data (including rules) when modal opens
  useEffect(() => {
    if (!open || !accessToken) return;

    const fetchFullPolicy = async () => {
      try {
        const fullPolicy = await getPolicy(accessToken, initialData.name);

        // Parse bitrate values
        const [bitrateUpValueStr, bitrateUpUnit] =
          fullPolicy.bitrate_uplink.split(" ");
        const [bitrateDownValueStr, bitrateDownUnit] =
          fullPolicy.bitrate_downlink.split(" ");

        setFormValues({
          name: fullPolicy.name,
          bitrateUpValue: parseInt(bitrateUpValueStr, 10),
          bitrateUpUnit: (bitrateUpUnit as "Mbps" | "Gbps") ?? "Mbps",
          bitrateDownValue: parseInt(bitrateDownValueStr, 10),
          bitrateDownUnit: (bitrateDownUnit as "Mbps" | "Gbps") ?? "Mbps",
          var5qi: fullPolicy.var5qi,
          arp: fullPolicy.arp,
          data_network_name: fullPolicy.data_network_name,
        });
        setErrors({});
        setTouched({});

        // Initialize rules from the full policy data
        setRulesLoading(true);
        const rules: InMemoryRule[] = [];

        // Add uplink rules
        if (fullPolicy.rules?.uplink) {
          fullPolicy.rules.uplink.forEach((rule, index) => {
            rules.push({
              description: rule.description,
              direction: "uplink",
              action: rule.action,
              remote_prefix: rule.remote_prefix,
              protocol: rule.protocol,
              port_low: rule.port_low,
              port_high: rule.port_high,
              precedence: (index + 1) * 100,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            });
          });
        }

        // Add downlink rules
        if (fullPolicy.rules?.downlink) {
          fullPolicy.rules.downlink.forEach((rule, index) => {
            rules.push({
              description: rule.description,
              direction: "downlink",
              action: rule.action,
              remote_prefix: rule.remote_prefix,
              protocol: rule.protocol,
              port_low: rule.port_low,
              port_high: rule.port_high,
              precedence: (index + 1) * 100,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            });
          });
        }

        setInMemoryRules(rules);
      } catch (error) {
        console.error("Failed to fetch policy data:", error);
      } finally {
        setRulesLoading(false);
      }
    };

    fetchFullPolicy();
  }, [open, initialData.name, accessToken]);

  // Fetch data networks
  useEffect(() => {
    const fetchDataNetworks = async () => {
      if (!open || !accessToken) return;
      try {
        const res: ListDataNetworksResponse = await listDataNetworks(
          accessToken,
          1,
          PER_PAGE,
        );
        setDataNetworks((res.items ?? []).map((dn) => dn.name));
      } catch (error) {
        console.error("Failed to fetch data networks:", error);
      }
    };
    fetchDataNetworks();
  }, [open, accessToken]);

  const filteredRules = inMemoryRules.filter(
    (rule) => rule.direction === activeTab,
  );

  // Policy form handlers
  const handleChange = (field: keyof FormState, value: string | number) => {
    setFormValues((prev) => ({ ...prev, [field]: value as never }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: string, value: string | number) => {
    try {
      await policySchema.validateAt(field, { ...formValues, [field]: value });
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

  const validateForm = useCallback(async () => {
    try {
      await policySchema.validate(formValues, { abortEarly: false });
      setErrors({});
      setIsValid(true);
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
      setIsValid(false);
    }
  }, [formValues]);

  useEffect(() => {
    validateForm();
  }, [validateForm, formValues]);

  const fiveQiIsAllowed = NON_GBR_5QI_VALUES.includes(formValues.var5qi);

  // Rule form handlers
  const resetRuleForm = (direction: Direction = "uplink") => {
    setRuleFormValues({
      description: "",
      direction,
      action: "allow",
      remotePrefix: "",
      protocol: "",
      portLow: "",
      portHigh: "",
    });
    setRuleErrors({});
    setRuleTouched({});
    setRuleFormAlert("");
    setEditingRuleId(null);
  };

  const validateRuleField = async (field: string, value: string) => {
    try {
      await ruleSchema.validateAt(field, { ...ruleFormValues, [field]: value });
      setRuleErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    } catch (err) {
      if (err instanceof ValidationError) {
        setRuleErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const validateRuleForm = async (): Promise<boolean> => {
    try {
      await ruleSchema.validate(ruleFormValues, { abortEarly: false });
      setRuleErrors({});
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
        setRuleErrors(validationErrors);
      }
      return false;
    }
  };

  const handleOpenCreateRuleForm = () => {
    resetRuleForm(activeTab);
    setFormDialogOpen(true);
  };

  const handleEditRule = (rule: InMemoryRule) => {
    setRuleFormValues({
      description: rule.description,
      direction: rule.direction,
      action: rule.action,
      remotePrefix: rule.remote_prefix || "",
      protocol: rule.protocol
        ? (PROTOCOL_NAMES[rule.protocol] ?? String(rule.protocol))
        : "",
      portLow: rule.port_low ? String(rule.port_low) : "",
      portHigh: rule.port_high ? String(rule.port_high) : "",
    });
    setRuleErrors({});
    setRuleTouched({});
    setRuleFormAlert("");
    setEditingRuleId(rule.id ?? rule.tempId ?? null);
    setFormDialogOpen(true);
  };

  const handleDeleteRule = (rule: InMemoryRule) => {
    setSelectedRuleId(rule.id ?? rule.tempId ?? null);
    setDeleteConfirmOpen(true);
  };

  const handleRuleFormChange = (field: keyof RuleFormValues, value: string) => {
    setRuleFormValues((prev) => ({ ...prev, [field]: value }));
    validateRuleField(field, value);
  };

  const handleRuleFormBlur = (field: string) => {
    setRuleTouched((prev) => ({ ...prev, [field]: true }));
  };

  const handleRuleFormSubmit = async () => {
    const isValid = await validateRuleForm();
    if (!isValid) return;

    setRuleFormAlert("");

    const protocol = ruleFormValues.protocol
      ? parseProtocol(ruleFormValues.protocol)
      : undefined;
    const portLow = ruleFormValues.portLow
      ? Number(ruleFormValues.portLow)
      : undefined;
    const portHigh = ruleFormValues.portHigh
      ? Number(ruleFormValues.portHigh)
      : undefined;
    const remotePrefix = ruleFormValues.remotePrefix || undefined;

    if (editingRuleId) {
      // Update existing rule in memory
      setInMemoryRules((prev) =>
        prev.map((r) => {
          if ((r.id ?? r.tempId) === editingRuleId) {
            return {
              ...r,
              description: ruleFormValues.description,
              direction: ruleFormValues.direction,
              action: ruleFormValues.action,
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
      // Add new rule in memory
      const newRule: InMemoryRule = {
        description: ruleFormValues.description,
        direction: ruleFormValues.direction,
        action: ruleFormValues.action,
        remote_prefix: remotePrefix,
        protocol: protocol ?? 0,
        port_low: portLow ?? 0,
        port_high: portHigh ?? 0,
        precedence:
          Math.max(...filteredRules.map((r) => r.precedence), 0) + 100,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        tempId: `temp-${Date.now()}`,
      };
      setInMemoryRules((prev) => [...prev, newRule]);
    }

    setFormDialogOpen(false);
    resetRuleForm();
  };

  const handleDeleteConfirm = async () => {
    if (selectedRuleId === null) return;
    setInMemoryRules((prev) =>
      prev.filter((r) => (r.id ?? r.tempId) !== selectedRuleId),
    );
    setDeleteConfirmOpen(false);
    setSelectedRuleId(null);
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

  // Main submit handler - update policy and apply all rule changes
  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    const bitrateUp = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
    const bitrateDown = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;

    try {
      // Build rules payload organized by direction
      const uplinkRules = inMemoryRules
        .filter((rule) => rule.direction === "uplink")
        .map((rule) => ({
          description: rule.description,
          remote_prefix: rule.remote_prefix,
          protocol: rule.protocol,
          port_low: rule.port_low,
          port_high: rule.port_high,
          action: rule.action,
        }));

      const downlinkRules = inMemoryRules
        .filter((rule) => rule.direction === "downlink")
        .map((rule) => ({
          description: rule.description,
          remote_prefix: rule.remote_prefix,
          protocol: rule.protocol,
          port_low: rule.port_low,
          port_high: rule.port_high,
          action: rule.action,
        }));

      const rules: PolicyRules = {};
      if (uplinkRules.length > 0) rules.uplink = uplinkRules;
      if (downlinkRules.length > 0) rules.downlink = downlinkRules;

      // Update the policy with all rules in a single request
      await updatePolicy(
        accessToken,
        formValues.name,
        bitrateUp,
        bitrateDown,
        formValues.var5qi,
        formValues.arp,
        formValues.data_network_name,
        Object.keys(rules).length > 0 ? rules : undefined,
      );

      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update policy: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

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

      const newRules = [...filteredRules];
      const [moved] = newRules.splice(sourceIndex, 1);
      newRules.splice(targetIndex, 0, moved);

      // Update all rules with new precedence
      setInMemoryRules((prev) => {
        const otherRules = prev.filter((r) => r.direction !== activeTab);
        return [
          ...otherRules,
          ...newRules.map((r, idx) => ({
            ...r,
            precedence: (idx + 1) * 100,
          })),
        ];
      });

      dragIndexRef.current = null;
    },
    [filteredRules, activeTab],
  );

  const handleDragEnd = useCallback(() => {
    dragIndexRef.current = null;
    setHoverIndex(null);
  }, []);

  return (
    <>
      <Dialog
        open={open}
        onClose={onClose}
        aria-labelledby="edit-policy-modal-title"
        aria-describedby="edit-policy-modal-description"
        fullWidth
        maxWidth="md"
      >
        <DialogTitle id="edit-policy-modal-title">Edit Policy</DialogTitle>
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

          {/* Policy Fields */}
          <TextField
            fullWidth
            label="Name"
            value={formValues.name}
            margin="normal"
            disabled
          />

          <FormControl fullWidth margin="normal">
            <InputLabel id="data-network-select-label">
              Data Network Name
            </InputLabel>
            <Select
              labelId="data-network-select-label"
              autoFocus
              label="Data Network Name"
              value={formValues.data_network_name}
              onChange={(e) =>
                handleChange("data_network_name", e.target.value)
              }
              onBlur={() => handleBlur("data_network_name")}
              error={!!errors.data_network_name && touched.data_network_name}
            >
              {dataNetworks.map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <Box display="flex" gap={2}>
            <TextField
              label="Bitrate Up Value"
              type="number"
              value={formValues.bitrateUpValue}
              onChange={(e) =>
                handleChange("bitrateUpValue", Number(e.target.value))
              }
              onBlur={() => handleBlur("bitrateUpValue")}
              error={!!errors.bitrateUpValue && touched.bitrateUpValue}
              helperText={touched.bitrateUpValue ? errors.bitrateUpValue : ""}
              margin="normal"
            />
            <TextField
              select
              label="Unit"
              value={formValues.bitrateUpUnit}
              onChange={(e) => handleChange("bitrateUpUnit", e.target.value)}
              onBlur={() => handleBlur("bitrateUpUnit")}
              margin="normal"
            >
              <MenuItem value="Mbps">Mbps</MenuItem>
              <MenuItem value="Gbps">Gbps</MenuItem>
            </TextField>
          </Box>

          <Box display="flex" gap={2}>
            <TextField
              label="Bitrate Down Value"
              type="number"
              value={formValues.bitrateDownValue}
              onChange={(e) =>
                handleChange("bitrateDownValue", Number(e.target.value))
              }
              onBlur={() => handleBlur("bitrateDownValue")}
              error={!!errors.bitrateDownValue && touched.bitrateDownValue}
              helperText={
                touched.bitrateDownValue ? errors.bitrateDownValue : ""
              }
              margin="normal"
            />
            <TextField
              select
              label="Unit"
              value={formValues.bitrateDownUnit}
              onChange={(e) => handleChange("bitrateDownUnit", e.target.value)}
              onBlur={() => handleBlur("bitrateDownUnit")}
              margin="normal"
            >
              <MenuItem value="Mbps">Mbps</MenuItem>
              <MenuItem value="Gbps">Gbps</MenuItem>
            </TextField>
          </Box>

          <FormControl fullWidth margin="normal">
            <InputLabel id="fiveqi-edit-select-label">5QI (non-GBR)</InputLabel>
            <Select
              labelId="fiveqi-edit-select-label"
              label="5QI (non-GBR)"
              value={formValues.var5qi}
              onChange={(e) => handleChange("var5qi", Number(e.target.value))}
              onBlur={() => handleBlur("var5qi")}
              error={!!errors.var5qi && touched.var5qi}
            >
              {!fiveQiIsAllowed && (
                <MenuItem value={formValues.var5qi} disabled>
                  {formValues.var5qi} (current, unsupported)
                </MenuItem>
              )}
              {NON_GBR_5QI_OPTIONS.map((opt) => (
                <MenuItem key={opt.value} value={opt.value}>
                  {opt.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <TextField
            fullWidth
            label="Allocation and Retention Priority (ARP)"
            type="number"
            value={formValues.arp}
            onChange={(e) => handleChange("arp", Number(e.target.value))}
            onBlur={() => handleBlur("arp")}
            error={!!errors.arp && touched.arp}
            helperText={
              touched.arp && errors.arp
                ? errors.arp
                : "1 (highest) to 15 (lowest)"
            }
            margin="normal"
          />

          {/* Rules Section */}
          <Typography variant="h6" sx={{ mt: 3, mb: 2 }}>
            Network Rules
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Rules are evaluated in order, top to bottom. The first matching rule
            is applied. Changes will be saved when you click Update.
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

          {rulesLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", p: 3 }}>
              <CircularProgress />
            </Box>
          ) : filteredRules.length === 0 ? (
            <Typography color="text.secondary" sx={{ p: 2 }}>
              No network rules configured for {activeTab} direction.
            </Typography>
          ) : (
            <List disablePadding>
              {filteredRules.map((rule: InMemoryRule, index: number) => (
                <ListItem
                  key={rule.id ?? rule.tempId}
                  draggable
                  onDragStart={handleDragStart(index)}
                  onDragOver={handleDragOver(index)}
                  onDrop={handleDrop(index)}
                  onDragEnd={handleDragEnd}
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
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

          {/* Add Rule Button */}
          <Box sx={{ mt: 2, display: "flex", justifyContent: "flex-end" }}>
            <Button
              variant="outlined"
              startIcon={<AddIcon />}
              onClick={handleOpenCreateRuleForm}
              size="small"
            >
              Add Rule
            </Button>
          </Box>
        </DialogContent>

        <DialogActions>
          <Button onClick={onClose}>Cancel</Button>
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
          <Collapse in={!!ruleFormAlert}>
            <Alert
              onClose={() => setRuleFormAlert("")}
              severity="error"
              sx={{ mb: 2 }}
            >
              {ruleFormAlert}
            </Alert>
          </Collapse>

          <TextField
            fullWidth
            label="Description"
            value={ruleFormValues.description}
            onChange={(e) =>
              handleRuleFormChange("description", e.target.value)
            }
            onBlur={() => handleRuleFormBlur("description")}
            error={!!ruleErrors.description && ruleTouched.description}
            helperText={ruleTouched.description ? ruleErrors.description : ""}
            margin="normal"
            autoFocus
          />

          <FormControl fullWidth margin="normal">
            <InputLabel id="direction-label">Direction</InputLabel>
            <Select
              labelId="direction-label"
              label="Direction"
              value={ruleFormValues.direction}
              onChange={(e) =>
                handleRuleFormChange("direction", e.target.value)
              }
              onBlur={() => handleRuleFormBlur("direction")}
              error={!!ruleErrors.direction && ruleTouched.direction}
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
              value={ruleFormValues.action}
              onChange={(e) => handleRuleFormChange("action", e.target.value)}
              onBlur={() => handleRuleFormBlur("action")}
              error={!!ruleErrors.action && ruleTouched.action}
            >
              <MenuItem value="allow">Allow</MenuItem>
              <MenuItem value="deny">Deny</MenuItem>
            </Select>
          </FormControl>

          <TextField
            fullWidth
            label="Remote Prefix (CIDR)"
            placeholder="e.g., 192.168.0.0/24"
            value={ruleFormValues.remotePrefix}
            onChange={(e) =>
              handleRuleFormChange("remotePrefix", e.target.value)
            }
            onBlur={() => handleRuleFormBlur("remotePrefix")}
            error={!!ruleErrors.remotePrefix && ruleTouched.remotePrefix}
            helperText={
              ruleTouched.remotePrefix ? ruleErrors.remotePrefix : "Optional"
            }
            margin="normal"
          />

          <Autocomplete
            fullWidth
            options={getProtocolOptions()}
            getOptionLabel={(option) => option.label}
            value={
              getProtocolOptions().find(
                (opt) => opt.value === ruleFormValues.protocol,
              ) ?? null
            }
            onChange={(_, value) => {
              handleRuleFormChange("protocol", value?.value ?? "");
            }}
            onBlur={() => handleRuleFormBlur("protocol")}
            isOptionEqualToValue={(option, value) =>
              option.value === value.value
            }
            renderInput={(params) => (
              <TextField
                {...params}
                label="Protocol"
                placeholder="Search protocols..."
                error={!!ruleErrors.protocol && ruleTouched.protocol}
                helperText={
                  ruleTouched.protocol
                    ? ruleErrors.protocol
                    : "Optional – search or leave empty for any"
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
              value={ruleFormValues.portLow}
              onChange={(e) => handleRuleFormChange("portLow", e.target.value)}
              onBlur={() => handleRuleFormBlur("portLow")}
              error={!!ruleErrors.portLow && ruleTouched.portLow}
              helperText={ruleTouched.portLow ? ruleErrors.portLow : "Optional"}
              margin="normal"
              sx={{ flex: 1 }}
            />
            <TextField
              label="Port High"
              type="number"
              placeholder="0-65535"
              value={ruleFormValues.portHigh}
              onChange={(e) => handleRuleFormChange("portHigh", e.target.value)}
              onBlur={() => handleRuleFormBlur("portHigh")}
              error={!!ruleErrors.portHigh && ruleTouched.portHigh}
              helperText={
                ruleTouched.portHigh ? ruleErrors.portHigh : "Optional"
              }
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
            onClick={handleRuleFormSubmit}
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

export default EditPolicyModal;
