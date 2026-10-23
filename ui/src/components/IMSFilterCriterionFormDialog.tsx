// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Controller,
  useFieldArray,
  useForm,
  useFormContext,
  useWatch,
} from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormGroup,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  Radio,
  RadioGroup,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  ExpandMore as ExpandMoreIcon,
} from "@mui/icons-material";
import {
  sessionCaseLabels,
  type IMSApplicationServer,
  type IMSInitialFilterCriterionInput,
  type IMSRegistrationType,
  type IMSSessionCase,
} from "@/queries/ims";
import {
  allRegistrationTypes,
  criterionFromValues,
  criterionToValues,
  emptyCondition,
  isRegisterMethod,
  sipMethods,
  type ConditionOperator,
  type ConditionType,
  type ConditionValues,
  type CriterionValues,
} from "@/utils/imsFilterCriteria";
import FormDialog from "@/components/form/FormDialog";
import SelectControl from "@/components/form/SelectControl";
import TextControl from "@/components/form/TextControl";

const conditionTypeLabels: Record<ConditionType, string> = {
  method: "SIP method",
  request_uri: "Request-URI",
  sip_header: "SIP header",
  session_case: "Session case",
  session_description: "SDP line",
};

const operatorLabels = (
  type: ConditionType,
): Partial<Record<ConditionOperator, string>> => {
  switch (type) {
    case "method":
    case "session_case":
      return { is: "is", is_not: "is not" };
    case "request_uri":
      return { is: "matches", is_not: "does not match" };
    default:
      return {
        is: "matches",
        is_not: "does not match",
        present: "is present",
        absent: "is absent",
      };
  }
};

const registrationTypeLabels: Record<IMSRegistrationType, string> = {
  INITIAL_REGISTRATION: "Initial registration",
  "RE-REGISTRATION": "Re-registration",
  "DE-REGISTRATION": "De-registration",
};

const sessionCases = Object.keys(sessionCaseLabels) as IMSSessionCase[];

const isRegex = (value: string) => {
  try {
    return !!new RegExp(value);
  } catch {
    return false;
  }
};

const regex = (label: string, applies: (c: ConditionValues) => boolean) =>
  yup
    .string()
    .default("")
    .test("required", `Enter ${label}`, function (v) {
      return !applies(this.parent as ConditionValues) || !!v;
    })
    .test("regex", "Must be a regular expression", function (v) {
      return !applies(this.parent as ConditionValues) || !v || isRegex(v);
    });

const hasContent = (c: ConditionValues) =>
  c.operator === "is" || c.operator === "is_not";

const conditionSchema = yup.object({
  type: yup
    .mixed<ConditionType>()
    .oneOf(Object.keys(conditionTypeLabels) as ConditionType[])
    .required(),
  operator: yup
    .mixed<ConditionOperator>()
    .oneOf(["is", "is_not", "present", "absent"])
    .required(),
  method: yup
    .string()
    .default("")
    .test("required", "Enter a SIP method", function (v) {
      return (this.parent as ConditionValues).type !== "method" || !!v?.trim();
    })
    .test("token", "A SIP method has no spaces", function (v) {
      return (
        (this.parent as ConditionValues).type !== "method" ||
        !/\s/.test(v?.trim() ?? "")
      );
    }),
  registration_types: yup
    .array()
    .of(yup.mixed<IMSRegistrationType>().oneOf(allRegistrationTypes).required())
    .default([])
    .test("one", "Select at least one", function (v) {
      return !isRegisterMethod(this.parent as ConditionValues) || !!v?.length;
    }),
  request_uri: regex("a pattern", (c) => c.type === "request_uri"),
  header: regex("a header name", (c) => c.type === "sip_header"),
  header_content: regex(
    "a value",
    (c) => c.type === "sip_header" && hasContent(c),
  ),
  session_case: yup.mixed<IMSSessionCase>().oneOf(sessionCases).required(),
  line: regex("an SDP line type", (c) => c.type === "session_description"),
  line_content: regex(
    "a value",
    (c) => c.type === "session_description" && hasContent(c),
  ),
});

const schema = yup.object({
  application_server_name: yup
    .string()
    .required("Select an application server"),
  default_handling: yup
    .mixed<CriterionValues["default_handling"]>()
    .oneOf(["SESSION_CONTINUED", "SESSION_TERMINATED"])
    .required(),
  always: yup.boolean().required(),
  conditions_match: yup
    .mixed<CriterionValues["conditions_match"]>()
    .oneOf(["all", "any"])
    .required(),
  groups: yup
    .array()
    .of(
      yup.object({
        conditions: yup.array().of(conditionSchema).required(),
      }),
    )
    .required()
    .when("always", ([always], s) => (always ? yup.array().required() : s)),
  profile_part: yup
    .mixed<CriterionValues["profile_part"]>()
    .oneOf(["BOTH", "REGISTERED", "UNREGISTERED"])
    .defined(),
  service_info: yup.string().defined(),
  include_register_request: yup.boolean().required(),
  include_register_response: yup.boolean().required(),
});

const ConditionRow: React.FC<{
  group: number;
  index: number;
  inGroup: boolean;
  onRemove?: () => void;
}> = ({ group, index, inGroup, onRemove }) => {
  const { control, setValue } = useFormContext<CriterionValues>();
  const path = `groups.${group}.conditions.${index}` as const;
  const condition = useWatch({ control, name: path });
  const operators = operatorLabels(condition.type);
  const withContent = hasContent(condition);
  const label = `Condition ${index + 1}${inGroup ? ` of group ${group + 1}` : ""}`;

  return (
    <Box
      component="fieldset"
      aria-label={label}
      sx={{ border: 0, m: 0, p: 0, py: 0.5, minWidth: 0 }}
    >
      <Box
        sx={{
          display: "flex",
          flexWrap: { xs: "wrap", sm: "nowrap" },
          gap: 1,
          alignItems: "flex-start",
        }}
      >
        <Controller
          control={control}
          name={`${path}.type`}
          render={({ field }) => (
            <TextField
              select
              size="small"
              label="Field"
              value={field.value}
              onChange={(e) => {
                const type = e.target.value as ConditionType;
                field.onChange(type);
                if (!(condition.operator in operatorLabels(type)))
                  setValue(`${path}.operator`, "is");
              }}
              sx={{ width: { xs: "100%", sm: 180 }, flexShrink: 0 }}
            >
              {Object.entries(conditionTypeLabels).map(([value, text]) => (
                <MenuItem key={value} value={value}>
                  {text}
                </MenuItem>
              ))}
            </TextField>
          )}
        />
        <Controller
          control={control}
          name={`${path}.operator`}
          render={({ field }) => (
            <TextField
              select
              size="small"
              label="Operator"
              value={field.value}
              onChange={(e) => field.onChange(e.target.value)}
              sx={{ width: { xs: "100%", sm: 150 }, flexShrink: 0 }}
            >
              {Object.entries(operators).map(([value, text]) => (
                <MenuItem key={value} value={value}>
                  {text}
                </MenuItem>
              ))}
            </TextField>
          )}
        />
        <Box sx={{ display: "flex", gap: 1, flex: 1, minWidth: 0 }}>
          {condition.type === "method" && (
            <Controller
              control={control}
              name={`${path}.method`}
              render={({ field, fieldState }) => (
                <Autocomplete
                  freeSolo
                  fullWidth
                  size="small"
                  options={sipMethods}
                  value={field.value}
                  inputValue={field.value}
                  onInputChange={(_e, v) => field.onChange(v)}
                  onBlur={field.onBlur}
                  renderInput={(params) => (
                    <TextField
                      {...params}
                      label="Method"
                      error={!!fieldState.error && fieldState.isTouched}
                      helperText={
                        fieldState.isTouched
                          ? fieldState.error?.message
                          : undefined
                      }
                    />
                  )}
                />
              )}
            />
          )}
          {condition.type === "request_uri" && (
            <TextControl<CriterionValues>
              name={`${path}.request_uri`}
              label="Pattern (regex)"
              size="small"
              margin="none"
              placeholder="^conference\.ims\.example\.org$"
              helperText="Matched against the host of a SIP URI or the number of a tel URI"
            />
          )}
          {condition.type === "sip_header" && (
            <>
              <TextControl<CriterionValues>
                name={`${path}.header`}
                label="Header name (regex)"
                size="small"
                margin="none"
                placeholder="^P-Asserted-Service$"
              />
              {withContent && (
                <TextControl<CriterionValues>
                  name={`${path}.header_content`}
                  label="Value (regex)"
                  size="small"
                  margin="none"
                />
              )}
            </>
          )}
          {condition.type === "session_case" && (
            <Controller
              control={control}
              name={`${path}.session_case`}
              render={({ field }) => (
                <TextField
                  select
                  fullWidth
                  size="small"
                  label="Session case"
                  value={field.value}
                  onChange={(e) => field.onChange(e.target.value)}
                >
                  {sessionCases.map((c) => (
                    <MenuItem key={c} value={c}>
                      {sessionCaseLabels[c]}
                    </MenuItem>
                  ))}
                </TextField>
              )}
            />
          )}
          {condition.type === "session_description" && (
            <>
              <TextControl<CriterionValues>
                name={`${path}.line`}
                label="Line type (regex)"
                size="small"
                margin="none"
                placeholder="^m$"
              />
              {withContent && (
                <TextControl<CriterionValues>
                  name={`${path}.line_content`}
                  label="Value (regex)"
                  size="small"
                  margin="none"
                  placeholder="^video"
                />
              )}
            </>
          )}
        </Box>
        <Tooltip title="Remove condition" arrow>
          <span>
            <IconButton
              aria-label={`Remove ${label.toLowerCase()}`}
              onClick={onRemove}
              disabled={!onRemove}
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      {isRegisterMethod(condition) && (
        <Controller
          control={control}
          name={`${path}.registration_types`}
          render={({ field, fieldState }) => (
            <FormControl error={!!fieldState.error} sx={{ mt: 0.5, ml: 1 }}>
              <FormGroup row>
                {allRegistrationTypes.map((t) => (
                  <FormControlLabel
                    key={t}
                    label={registrationTypeLabels[t]}
                    control={
                      <Checkbox
                        size="small"
                        checked={field.value.includes(t)}
                        onBlur={field.onBlur}
                        onChange={(e) =>
                          field.onChange(
                            e.target.checked
                              ? [...field.value, t]
                              : field.value.filter((v) => v !== t),
                          )
                        }
                      />
                    }
                  />
                ))}
              </FormGroup>
              <FormHelperText sx={{ mx: 0 }}>
                {fieldState.error?.message ??
                  "S-CSCFs that don't support registration types match every REGISTER."}
              </FormHelperText>
            </FormControl>
          )}
        />
      )}
      {condition.type === "session_case" && condition.operator === "is_not" && (
        <Alert severity="warning" sx={{ mt: 1 }}>
          This also matches session cases that later 3GPP releases add.
        </Alert>
      )}
    </Box>
  );
};

const Connector: React.FC<{ word: string }> = ({ word }) => (
  <Typography
    variant="caption"
    color="textSecondary"
    sx={{ display: "block", fontWeight: 600, letterSpacing: 1, my: 0.5 }}
  >
    {word}
  </Typography>
);

const GroupEditor: React.FC<{
  group: number;
  inner: string;
  showHeader: boolean;
  onRemove: () => void;
}> = ({ group, inner, showHeader, onRemove }) => {
  const { control } = useFormContext<CriterionValues>();
  const { fields, append, remove } = useFieldArray({
    control,
    name: `groups.${group}.conditions`,
  });

  return (
    <Box
      sx={{
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
        px: 1.5,
        pt: showHeader ? 0.5 : 1.5,
        pb: 1,
      }}
    >
      {showHeader && (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography variant="subtitle2">Group {group + 1}</Typography>
          <Tooltip title="Remove group" arrow>
            <IconButton
              size="small"
              aria-label={`Remove group ${group + 1}`}
              onClick={onRemove}
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
      )}
      {fields.map((field, index) => (
        <React.Fragment key={field.id}>
          {index > 0 && <Connector word={inner} />}
          <ConditionRow
            group={group}
            index={index}
            inGroup={showHeader}
            onRemove={
              fields.length > 1
                ? () => remove(index)
                : showHeader
                  ? onRemove
                  : undefined
            }
          />
        </React.Fragment>
      ))}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={() => append(emptyCondition())}
        sx={{ mt: 0.5 }}
      >
        Add Condition
      </Button>
    </Box>
  );
};

const TriggerEditor: React.FC = () => {
  const { control } = useFormContext<CriterionValues>();
  const { fields, append, remove } = useFieldArray({
    control,
    name: "groups",
  });
  const always = useWatch({ control, name: "always" });
  const match = useWatch({ control, name: "conditions_match" });
  const inner = match === "all" ? "AND" : "OR";
  const outer = match === "all" ? "OR" : "AND";
  const multiple = fields.length > 1;

  const matchSelect = (
    <Controller
      control={control}
      name="conditions_match"
      render={({ field }) => (
        <TextField
          select
          size="small"
          value={field.value}
          onChange={(e) => field.onChange(e.target.value)}
          slotProps={{ htmlInput: { "aria-label": "Conditions match" } }}
          sx={{ mx: 1, minWidth: 80 }}
        >
          <MenuItem value="all">all</MenuItem>
          <MenuItem value="any">any</MenuItem>
        </TextField>
      )}
    />
  );

  return (
    <Box sx={{ mt: 2 }}>
      <Controller
        control={control}
        name="always"
        render={({ field }) => (
          <FormControl>
            <FormLabel id="ifc-trigger-label">Trigger</FormLabel>
            <RadioGroup
              row
              aria-labelledby="ifc-trigger-label"
              value={field.value ? "always" : "conditions"}
              onChange={(e) => field.onChange(e.target.value === "always")}
            >
              <FormControlLabel
                value="conditions"
                control={<Radio />}
                label="When conditions match"
              />
              <FormControlLabel
                value="always"
                control={<Radio />}
                label="Always"
              />
            </RadioGroup>
          </FormControl>
        )}
      />
      {always ? (
        <Typography variant="body2" color="textSecondary">
          Every request from or to the subscriber goes to the application
          server.
        </Typography>
      ) : (
        <>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              flexWrap: "wrap",
              my: 1,
            }}
          >
            <Typography variant="body2">
              {multiple ? "A group matches when" : "Match"}
            </Typography>
            {matchSelect}
            <Typography variant="body2">
              {multiple
                ? `of its conditions match. The criterion matches when ${match === "all" ? "any group matches" : "every group matches"}.`
                : "of these conditions"}
            </Typography>
          </Box>
          {fields.map((field, index) => (
            <React.Fragment key={field.id}>
              {index > 0 && (
                <Box sx={{ textAlign: "center" }}>
                  <Connector word={outer} />
                </Box>
              )}
              <GroupEditor
                group={index}
                inner={inner}
                showHeader={multiple}
                onRemove={() => remove(index)}
              />
            </React.Fragment>
          ))}
          <Button
            size="small"
            startIcon={<AddIcon />}
            onClick={() => append({ conditions: [emptyCondition()] })}
            sx={{ mt: 1 }}
          >
            Add Group
          </Button>
        </>
      )}
    </Box>
  );
};

const profilePartOptions = [
  { value: "BOTH", label: "Registered and unregistered subscribers" },
  { value: "REGISTERED", label: "Registered subscribers" },
  { value: "UNREGISTERED", label: "Unregistered subscribers" },
] as const;

interface IMSFilterCriterionFormDialogProps {
  open: boolean;
  onClose: () => void;
  onSave: (criterion: IMSInitialFilterCriterionInput) => void;
  criterion?: IMSInitialFilterCriterionInput;
  servers: IMSApplicationServer[];
}

const IMSFilterCriterionFormDialog: React.FC<
  IMSFilterCriterionFormDialogProps
> = ({ open, onClose, onSave, criterion, servers }) => {
  const initial = criterionToValues(criterion, servers[0]?.name ?? "");
  const form = useForm<CriterionValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: initial,
  });
  const advanced =
    initial.profile_part !== "BOTH" ||
    !!initial.service_info ||
    initial.include_register_request ||
    initial.include_register_response;

  const submit = async (values: CriterionValues) => {
    onSave(criterionFromValues(values));
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      title={criterion ? "Edit Filter Criterion" : "Add Filter Criterion"}
      form={form}
      onSubmit={submit}
      submitLabel={criterion ? "Update" : "Add"}
      submittingLabel={criterion ? "Update" : "Add"}
      maxWidth="md"
    >
      <SelectControl<CriterionValues, string>
        name="application_server_name"
        label="Application Server"
        options={servers.map((s) => ({
          value: s.name,
          label: `${s.name} (${s.server_name})`,
        }))}
      />
      <Controller
        control={form.control}
        name="default_handling"
        render={({ field }) => (
          <FormControl sx={{ mt: 1 }}>
            <FormLabel id="ifc-handling-label">
              If the application server can't be reached
            </FormLabel>
            <RadioGroup
              row
              aria-labelledby="ifc-handling-label"
              value={field.value}
              onChange={(e) => field.onChange(e.target.value)}
            >
              <FormControlLabel
                value="SESSION_CONTINUED"
                control={<Radio />}
                label="Continue the session"
              />
              <FormControlLabel
                value="SESSION_TERMINATED"
                control={<Radio />}
                label="Terminate the session"
              />
            </RadioGroup>
          </FormControl>
        )}
      />
      <TriggerEditor />
      <Accordion
        key={String(open)}
        defaultExpanded={advanced}
        disableGutters
        variant="outlined"
        sx={{ mt: 3, "&::before": { display: "none" } }}
      >
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Typography variant="subtitle2">Advanced</Typography>
        </AccordionSummary>
        <AccordionDetails sx={{ pt: 0 }}>
          <SelectControl<CriterionValues, string>
            name="profile_part"
            label="Applies To"
            options={profilePartOptions}
          />
          <FormGroup sx={{ mt: 1 }}>
            {(
              [
                [
                  "include_register_request",
                  "Include the REGISTER request in the third-party REGISTER",
                ],
                [
                  "include_register_response",
                  "Include the REGISTER response in the third-party REGISTER",
                ],
              ] as const
            ).map(([name, text]) => (
              <Controller
                key={name}
                control={form.control}
                name={name}
                render={({ field }) => (
                  <FormControlLabel
                    label={text}
                    control={
                      <Checkbox
                        checked={field.value}
                        onChange={(e) => field.onChange(e.target.checked)}
                      />
                    }
                  />
                )}
              />
            ))}
          </FormGroup>
          <TextControl<CriterionValues>
            name="service_info"
            label="Service Info"
            helperText="Sent unchanged to the application server."
          />
        </AccordionDetails>
      </Accordion>
    </FormDialog>
  );
};

export default IMSFilterCriterionFormDialog;
