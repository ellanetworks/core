// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useRef, useState } from "react";
import { Box, Button, Chip, IconButton, Typography } from "@mui/material";
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  DragIndicator as DragIcon,
} from "@mui/icons-material";
import { useController, useForm, useFormState } from "react-hook-form";
import {
  updatePolicy,
  type APIPolicy,
  type PolicyRule,
} from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import { PROTOCOL_NAMES } from "@/utils/formatters";
import IPProtocolChip from "@/components/IPProtocolChip";
import FormDialog from "@/components/form/FormDialog";
import PolicyRuleFormDialog, {
  EMPTY_RULE_FORM,
  parseProtocol,
  type PolicyRuleFormValues,
} from "@/components/PolicyRuleFormDialog";

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
  rules: InMemoryRule[];
}

const formatPorts = (rule: InMemoryRule): string => {
  if (rule.port_low === 0 && rule.port_high === 0) return "any";
  if (rule.port_low === rule.port_high) return String(rule.port_low);
  return `${rule.port_low}-${rule.port_high}`;
};

const toFormValues = (rule: InMemoryRule): PolicyRuleFormValues => ({
  description: rule.description,
  action: rule.action,
  remotePrefix: rule.remote_prefix || "",
  protocol:
    rule.protocol !== 0
      ? (PROTOCOL_NAMES[rule.protocol] ?? String(rule.protocol))
      : "",
  portLow: rule.port_low !== 0 ? String(rule.port_low) : "",
  portHigh: rule.port_high !== 0 ? String(rule.port_high) : "",
});

const fromFormValues = (values: PolicyRuleFormValues) => ({
  description: values.description,
  action: values.action as Action,
  remote_prefix: values.remotePrefix || undefined,
  protocol: (values.protocol ? parseProtocol(values.protocol) : undefined) ?? 0,
  port_low: values.portLow ? Number(values.portLow) : 0,
  port_high: values.portHigh ? Number(values.portHigh) : 0,
});

const toApiRules = (rules: InMemoryRule[]): PolicyRule[] =>
  rules.map((rule) => ({
    description: rule.description,
    remote_prefix: rule.remote_prefix,
    protocol: rule.protocol,
    port_low: rule.port_low,
    port_high: rule.port_high,
    action: rule.action,
  }));

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

  const form = useForm<FormValues>({
    defaultValues: {
      rules: directionRules.map((rule, index) => ({
        tempId: `rule-${index}`,
        description: rule.description,
        action: rule.action,
        remote_prefix: rule.remote_prefix,
        protocol: rule.protocol,
        port_low: rule.port_low,
        port_high: rule.port_high,
      })),
    },
  });

  const { field } = useController({ control: form.control, name: "rules" });
  const { isSubmitting } = useFormState({ control: form.control });
  const rules: InMemoryRule[] = field.value ?? [];

  const [ruleFormOpen, setRuleFormOpen] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [editingValues, setEditingValues] =
    useState<PolicyRuleFormValues>(EMPTY_RULE_FORM);
  const nextId = useRef(0);

  const dragIndexRef = useRef<number | null>(null);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const handleDragStart = (index: number) => (event: React.DragEvent) => {
    dragIndexRef.current = index;
    event.dataTransfer.effectAllowed = "move";
    const element = event.currentTarget as HTMLElement;
    event.dataTransfer.setDragImage(
      element,
      element.offsetWidth / 2,
      element.offsetHeight / 2,
    );
  };

  const handleDragOver = (index: number) => (event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setHoverIndex(index);
  };

  const handleDrop = (targetIndex: number) => (event: React.DragEvent) => {
    event.preventDefault();
    setHoverIndex(null);

    const sourceIndex = dragIndexRef.current;
    dragIndexRef.current = null;
    if (sourceIndex === null || sourceIndex === targetIndex) return;

    const next = [...rules];
    const [moved] = next.splice(sourceIndex, 1);
    next.splice(targetIndex, 0, moved);
    field.onChange(next);
  };

  const handleDragEnd = () => {
    dragIndexRef.current = null;
    setHoverIndex(null);
  };

  const openCreateForm = () => {
    setEditingRuleId(null);
    setEditingValues(EMPTY_RULE_FORM);
    setRuleFormOpen(true);
  };

  const openEditForm = (rule: InMemoryRule) => {
    setEditingRuleId(rule.tempId);
    setEditingValues(toFormValues(rule));
    setRuleFormOpen(true);
  };

  const saveRule = (values: PolicyRuleFormValues) => {
    const patch = fromFormValues(values);
    if (editingRuleId) {
      field.onChange(
        rules.map((rule) =>
          rule.tempId === editingRuleId ? { ...rule, ...patch } : rule,
        ),
      );
    } else {
      nextId.current += 1;
      field.onChange([...rules, { tempId: `new-${nextId.current}`, ...patch }]);
    }
    setRuleFormOpen(false);
  };

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;

    const otherDirection =
      direction === "uplink" ? policy.rules?.downlink : policy.rules?.uplink;
    const hasOther = !!otherDirection && otherDirection.length > 0;

    const updatedRules =
      direction === "uplink"
        ? {
            uplink: toApiRules(values.rules),
            ...(hasOther ? { downlink: otherDirection } : {}),
          }
        : {
            downlink: toApiRules(values.rules),
            ...(hasOther ? { uplink: otherDirection } : {}),
          };

    await updatePolicy(accessToken, policy.name, {
      profile_name: policy.profile_name,
      slice_name: policy.slice_name,
      data_network_name: policy.data_network_name,
      session_ambr_uplink: policy.session_ambr_uplink,
      session_ambr_downlink: policy.session_ambr_downlink,
      var5qi: policy.var5qi,
      arp: policy.arp,
      rules: updatedRules,
    });
  };

  const directionLabel = direction === "uplink" ? "Uplink" : "Downlink";

  return (
    <>
      <FormDialog
        open={open}
        onClose={onClose}
        onSuccess={onSuccess}
        title={`Edit ${directionLabel} Rules`}
        description="Rules are evaluated in order, top to bottom. The first matching rule is applied. Drag rows to reorder."
        form={form}
        onSubmit={submit}
        errorPrefix="Failed to save rules"
        submitLabel="Save"
        submittingLabel="Saving..."
        maxWidth="md"
        fullWidth
      >
        {rules.length === 0 ? (
          <Typography color="textSecondary" sx={{ p: 2 }}>
            No {direction} rules configured.
          </Typography>
        ) : (
          <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1 }}>
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
                  sx={{ fontWeight: 600, width: 24, flexShrink: 0 }}
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
                  <IPProtocolChip protocol={rule.protocol} />
                </Box>
                <Typography
                  variant="body2"
                  sx={{
                    minWidth: 220,
                    flex: "0 1 220px",
                    whiteSpace: "normal",
                    wordBreak: "break-word",
                    pr: 1,
                  }}
                >
                  {rule.remote_prefix || "any"}
                </Typography>
                <Typography
                  variant="body2"
                  color="textSecondary"
                  sx={{ width: 90, flexShrink: 0, whiteSpace: "nowrap" }}
                >
                  {formatPorts(rule)}
                </Typography>
                <Typography
                  variant="body2"
                  color="textSecondary"
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
                    onClick={() => openEditForm(rule)}
                    title="Edit rule"
                  >
                    <EditIcon fontSize="small" />
                  </IconButton>
                  <IconButton
                    size="small"
                    color="primary"
                    onClick={() =>
                      field.onChange(
                        rules.filter((r) => r.tempId !== rule.tempId),
                      )
                    }
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
            onClick={openCreateForm}
            disabled={isSubmitting}
            size="small"
          >
            Add Rule
          </Button>
        </Box>
      </FormDialog>

      <PolicyRuleFormDialog
        open={ruleFormOpen}
        onClose={() => setRuleFormOpen(false)}
        onSave={saveRule}
        initialValues={editingValues}
        isEditing={!!editingRuleId}
      />
    </>
  );
};

export default PolicyRulesModal;
