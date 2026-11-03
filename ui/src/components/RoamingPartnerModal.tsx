// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo } from "react";
import {
  Box,
  Button,
  Collapse,
  FormControlLabel,
  FormHelperText,
  IconButton,
  Switch,
  Tooltip,
  Typography,
} from "@mui/material";
import { Add as AddIcon, Delete as DeleteIcon } from "@mui/icons-material";
import {
  useFieldArray,
  useForm,
  useController,
  useWatch,
} from "react-hook-form";
import type { Control, Path } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import { Link } from "react-router-dom";
import * as yup from "yup";
import {
  createRoamingPartner,
  defaultNRFAPIRoot,
  diameterPeerLabel,
  isWellKnownSEPPHost,
  parseSEPPFQDN,
  samePLMN,
  updateRoamingPartner,
  type PLMNID,
  type RoamingDiameterPeer,
  type RoamingPartner,
  type RoamingPartnerInput,
} from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";
import PemControl from "@/components/form/PemControl";
import ProfileSelectField, {
  useProfileNames,
} from "@/components/ProfileSelectField";
import { PRODUCT } from "@/utils/product";

interface RoamingPartnerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (partner: RoamingPartnerInput) => void;
  partner?: RoamingPartner;
  diameterPeers: RoamingDiameterPeer[];
}

const PEM_CERTIFICATE_RE =
  /-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----/;

export const isAPIRoot = (value: string, schemes: string[]): boolean => {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return false;
  }
  return (
    schemes.includes(url.protocol.replace(/:$/, "")) &&
    !!url.hostname &&
    !url.username &&
    !url.password &&
    !url.search &&
    !url.hash &&
    !value.includes("?") &&
    !value.includes("#")
  );
};

const when5G = (
  test: (value: string) => boolean,
): yup.TestFunction<string | undefined> =>
  function (value) {
    if (!this.parent.has5G) return true;
    return test((value ?? "").trim());
  };

const isPLMN = (p: { mcc?: string; mnc?: string }): p is PLMNID =>
  /^\d{3}$/.test(p.mcc ?? "") && /^\d{2,3}$/.test(p.mnc ?? "");

export const plmnsConflict = (a: PLMNID, b: PLMNID) =>
  a.mcc === b.mcc &&
  (samePLMN(a, b) || a.mnc.startsWith(b.mnc) || b.mnc.startsWith(a.mnc));

const PLMNS_CONFLICT =
  "Two PLMNs are the same or ambiguous (same MCC, one MNC a prefix of the other)";

const SEPP_FQDN_FORMAT =
  "SEPP FQDN must be <host>.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org or <host>.5gc.mnc<MNC>.mcc<MCC>.<IPX>.ipxnetwork.org, for example sepp1.5gc.mnc001.mcc001.3gppnetwork.org";

export const schema = yup.object({
  name: yup
    .string()
    .default("")
    .test("required", "Name is required", (v) => !!v?.trim()),
  plmns: yup
    .array()
    .of(
      yup.object({
        mcc: yup
          .string()
          .default("")
          .matches(/^\d{3}$/, "MCC must be 3 digits")
          .required("MCC is required"),
        mnc: yup
          .string()
          .default("")
          .matches(/^\d{2,3}$/, "MNC must be 2 or 3 digits")
          .required("MNC is required"),
      }),
    )
    .default([])
    .min(1, "Add at least one PLMN")
    .test("unique", PLMNS_CONFLICT, (rows) => {
      const plmns = (rows ?? []).filter(isPLMN);
      return plmns.every((a, i) =>
        plmns.slice(i + 1).every((b) => !plmnsConflict(a, b)),
      );
    }),
  profileName: yup.string().default("").required("Profile is required"),
  has5G: yup.boolean().default(false),
  seppFQDN: yup
    .string()
    .default("")
    .test("required", "SEPP FQDN is required", when5G(Boolean))
    .test(
      "fqdn",
      SEPP_FQDN_FORMAT,
      when5G((v) => !v || parseSEPPFQDN(v) !== null),
    )
    .test(
      "well-known",
      "SEPP FQDN must not be the well-known sepp.5gc… name: use the partner SEPP's own name, for example sepp1.5gc…",
      when5G((v) => {
        const parts = v ? parseSEPPFQDN(v) : null;
        return !parts || !isWellKnownSEPPHost(parts.host);
      }),
    )
    .test(
      "plmn",
      "SEPP FQDN must name one of the partner's PLMNs",
      function (value) {
        if (!this.parent.has5G) return true;
        const parts = value ? parseSEPPFQDN(value.trim()) : null;
        if (!parts) return true;
        const plmns = ((this.parent.plmns ?? []) as PLMNID[]).filter(isPLMN);
        return plmns.some((p) => samePLMN(p, parts.plmn));
      },
    ),
  n32cURI: yup
    .string()
    .default("")
    .test("required", "N32-c URI is required", when5G(Boolean))
    .test(
      "uri",
      "N32-c URI must be an https URI with no query, for example https://sepp.example.org:443",
      when5G((v) => !v || isAPIRoot(v, ["https"])),
    ),
  n32fURI: yup
    .string()
    .default("")
    .test(
      "uri",
      "N32-f URI must be an https URI with no query",
      when5G((v) => !v || isAPIRoot(v, ["https"])),
    ),
  trustAnchorPEM: yup
    .string()
    .default("")
    .test("required", "Trust anchor is required", when5G(Boolean))
    .test(
      "pem",
      "Trust anchor must be one or more PEM certificates",
      when5G((v) => !v || PEM_CERTIFICATE_RE.test(v)),
    ),
  nrfAPIRoot: yup
    .string()
    .default("")
    .test(
      "uri",
      "NRF API root must be an http or https URI with no query",
      when5G((v) => !v || isAPIRoot(v, ["http", "https"])),
    ),
  has4G: yup.boolean().default(false),
  diameterPeerID: yup
    .string()
    .default("")
    .test("required", "Diameter peer is required", function (value) {
      return !this.parent.has4G || !!value;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const toFormValues = (partner?: RoamingPartner): FormValues => ({
  name: partner?.name ?? "",
  plmns: partner?.plmns.map(({ mcc, mnc }) => ({ mcc, mnc })) ?? [
    { mcc: "", mnc: "" },
  ],
  profileName: partner?.profileName ?? "",
  has5G: !!partner?.sepp,
  seppFQDN: partner?.sepp?.fqdn ?? "",
  n32cURI: partner?.sepp?.n32cURI ?? "",
  n32fURI: partner?.sepp?.n32fURI ?? "",
  trustAnchorPEM: partner?.sepp?.trustAnchorPEM ?? "",
  nrfAPIRoot: partner?.sepp?.nrfAPIRoot ?? "",
  has4G: !!partner?.diameterPeerID,
  diameterPeerID: partner?.diameterPeerID ?? "",
});

export const toPartnerInput = (values: FormValues): RoamingPartnerInput => {
  const input: RoamingPartnerInput = {
    name: values.name.trim(),
    plmns: values.plmns.map(({ mcc, mnc }) => ({ mcc, mnc })),
    profileName: values.profileName,
  };
  if (values.has5G) {
    const sepp: RoamingPartnerInput["sepp"] = {
      fqdn: values.seppFQDN.trim(),
      n32cURI: values.n32cURI.trim(),
      trustAnchorPEM: values.trustAnchorPEM.trim() + "\n",
    };
    if (values.n32fURI.trim()) sepp.n32fURI = values.n32fURI.trim();
    if (values.nrfAPIRoot.trim()) sepp.nrfAPIRoot = values.nrfAPIRoot.trim();
    input.sepp = sepp;
  }
  if (values.has4G && values.diameterPeerID) {
    input.diameterPeerID = values.diameterPeerID;
  }
  return input;
};

const SectionSwitch = ({
  control,
  name,
  label,
  helperText,
}: {
  control: Control<FormValues>;
  name: Path<FormValues>;
  label: string;
  helperText: string;
}) => {
  const { field } = useController({ control, name });
  return (
    <Box sx={{ mt: 2 }}>
      <FormControlLabel
        control={
          <Switch
            checked={!!field.value}
            onChange={(_, on) => field.onChange(on)}
          />
        }
        label={label}
      />
      <FormHelperText sx={{ mt: 0 }}>{helperText}</FormHelperText>
    </Box>
  );
};

const RoamingPartnerModal: React.FC<RoamingPartnerModalProps> = ({
  open,
  onClose,
  onSuccess,
  partner,
  diameterPeers,
}) => {
  const { accessToken } = useAuth();
  const profilesQuery = useProfileNames(open);
  const profiles = useMemo(
    () => profilesQuery.data ?? [],
    [profilesQuery.data],
  );

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: toFormValues(partner),
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "plmns",
  });

  const [has5G, has4G, plmns, seppFQDN] = useWatch({
    control: form.control,
    name: ["has5G", "has4G", "plmns", "seppFQDN"],
  });

  const nrfPLMN =
    parseSEPPFQDN((seppFQDN ?? "").trim())?.plmn ?? (plmns ?? []).find(isPLMN);
  const nrfPlaceholder = nrfPLMN
    ? defaultNRFAPIRoot(nrfPLMN.mcc, nrfPLMN.mnc)
    : "https://nrf.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org";

  const validPLMNs = (plmns ?? []).filter(isPLMN);
  const plmnsError = validPLMNs.some((a, i) =>
    validPLMNs.slice(i + 1).some((b) => plmnsConflict(a, b)),
  )
    ? PLMNS_CONFLICT
    : undefined;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const input = toPartnerInput(values);
    if (partner) {
      await updateRoamingPartner(accessToken, partner.id, input);
    } else {
      await createRoamingPartner(accessToken, input);
    }
  };

  const peerOptions = diameterPeers.map((peer) => ({
    value: peer.id,
    label: diameterPeerLabel(peer),
  }));

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={() => onSuccess(toPartnerInput(form.getValues()))}
      title={partner ? "Edit Roaming Partner" : "Add Roaming Partner"}
      description="A roaming agreement with a home network whose subscribers may attach here. The profile sets the local policy applied to its roamers."
      form={form}
      onSubmit={submit}
      errorPrefix={
        partner
          ? "Failed to update roaming partner"
          : "Failed to add roaming partner"
      }
      submitLabel={partner ? "Update" : "Add"}
      submittingLabel={partner ? "Updating..." : "Adding..."}
      maxWidth="md"
    >
      <TextControl<FormValues>
        name="name"
        label="Name"
        placeholder="Partner Telecom"
        autoFocus
      />
      <Typography variant="subtitle2" sx={{ mt: 2 }}>
        PLMNs
      </Typography>
      <Typography variant="body2" color="textSecondary">
        The partner&apos;s PLMN IDs. Roamers are the IMSIs that start with one
        of them: the MNC length matters. A PLMN that still has roamers cannot be
        removed.
      </Typography>
      {fields.map((field, index) => (
        <Box
          key={field.id}
          sx={{ display: "flex", gap: 2, alignItems: "center" }}
        >
          <TextControl<FormValues>
            name={`plmns.${index}.mcc`}
            label={`MCC ${index + 1}`}
            placeholder="001"
          />
          <TextControl<FormValues>
            name={`plmns.${index}.mnc`}
            label={`MNC ${index + 1}`}
            placeholder="01"
          />
          <Tooltip title="Remove PLMN" arrow>
            <span>
              <IconButton
                aria-label={`Remove PLMN ${index + 1}`}
                onClick={() => remove(index)}
                disabled={fields.length === 1}
              >
                <DeleteIcon
                  fontSize="small"
                  color={fields.length === 1 ? "disabled" : "primary"}
                />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      ))}
      {plmnsError && (
        <Typography variant="body2" color="error" sx={{ mt: 1 }}>
          {plmnsError}
        </Typography>
      )}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={() => append({ mcc: "", mnc: "" })}
        sx={{ mt: 1, mb: 1 }}
      >
        Add PLMN
      </Button>
      <ProfileSelectField
        control={form.control}
        name="profileName"
        profiles={profiles}
      />
      <FormHelperText>
        The profile applied to this partner&apos;s roamers: its policies set
        which slices and data networks they may use.
      </FormHelperText>

      <SectionSwitch
        control={form.control}
        name="has5G"
        label="5G roaming"
        helperText="The partner's SEPP, reached over N32."
      />
      <Collapse in={!!has5G} unmountOnExit>
        <TextControl<FormValues>
          name="seppFQDN"
          label="SEPP FQDN"
          placeholder="sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
          helperText="The name the partner SEPP's certificate carries, under the domain of one of its PLMNs."
        />
        <TextControl<FormValues>
          name="n32cURI"
          label="N32-c URI"
          placeholder="https://sepp1.example.org:443"
          helperText="Where the partner SEPP accepts the N32 handshake."
        />
        <TextControl<FormValues>
          name="n32fURI"
          label="N32-f URI"
          placeholder="Same as N32-c"
          helperText="Optional. Only if the partner SEPP forwards messages at a different address."
        />
        <PemControl<FormValues>
          name="trustAnchorPEM"
          label="Trust Anchor"
          helperText="PEM certificates the partner SEPP's certificate chains to, from the partner. A self-signed partner certificate may be its own trust anchor."
          accept=".pem,.crt,.cer"
        />
        <TextControl<FormValues>
          name="nrfAPIRoot"
          label="NRF API Root"
          placeholder={nrfPlaceholder}
          helperText="Optional. The partner's NRF, used to find its AUSF and UDM. Leave empty for the default."
        />
        <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
          Give the partner {PRODUCT.name}&apos;s trust anchor from the{" "}
          <Link to="/roaming/sepp">SEPP</Link> tab. {PRODUCT.name} generates its
          SEPP certificate with the first 5G partner.
        </Typography>
      </Collapse>

      <SectionSwitch
        control={form.control}
        name="has4G"
        label="4G roaming"
        helperText="The partner's HSS, reached over S6a through a Diameter peer."
      />
      <Collapse in={!!has4G} unmountOnExit>
        {peerOptions.length > 0 ? (
          <SelectControl<FormValues, string>
            name="diameterPeerID"
            label="Diameter Peer"
            options={peerOptions}
          />
        ) : (
          <Typography variant="body2" color="textSecondary" sx={{ mt: 2 }}>
            No roaming Diameter peers yet. Add one in the{" "}
            <Link to="/roaming/diameter-peers">Diameter Peers</Link> tab first.
          </Typography>
        )}
      </Collapse>
    </FormDialog>
  );
};

export default RoamingPartnerModal;
