// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useRef } from "react";
import { Alert } from "@mui/material";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  defaultSEPPFQDN,
  generateSEPPCertificate,
  importSEPPCertificate,
  isGeneratedSEPPHost,
  isWellKnownSEPPHost,
  plmnDomain,
  type SEPPCertificate,
} from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import PemControl from "@/components/form/PemControl";
import { diameterIdentityRegex } from "@/components/SMSCPeerModal";
import { PRODUCT } from "@/utils/product";

const PEM_BLOCK_RE = (type: string) =>
  new RegExp(`-----BEGIN ${type}-----[\\s\\S]+?-----END ${type}-----`);

const importSchema = yup.object({
  certificateChainPEM: yup
    .string()
    .default("")
    .test("required", "Certificate chain is required", (v) => !!v?.trim())
    .test(
      "pem",
      "Certificate chain must be PEM certificates",
      (v) => !v?.trim() || PEM_BLOCK_RE("CERTIFICATE").test(v),
    ),
  privateKeyPEM: yup
    .string()
    .default("")
    .test("required", "Private key is required", (v) => !!v?.trim())
    .test(
      "pem",
      "Private key must be a PEM private key",
      (v) => !v?.trim() || PEM_BLOCK_RE("(?:EC |RSA )?PRIVATE KEY").test(v),
    ),
  fqdn: yup
    .string()
    .default("")
    .test(
      "fqdn",
      "FQDN must be a fully qualified domain name",
      (v) => !v?.trim() || diameterIdentityRegex.test(v.trim()),
    ),
});

type ImportValues = yup.InferType<typeof importSchema>;

interface ImportSEPPCertificateModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (cert: SEPPCertificate) => void;
  mcc?: string;
  mnc?: string;
}

export const ImportSEPPCertificateModal: React.FC<
  ImportSEPPCertificateModalProps
> = ({ open, onClose, onSuccess, mcc, mnc }) => {
  const { accessToken } = useAuth();
  const form = useForm<ImportValues>({
    mode: "onTouched",
    resolver: yupResolver(importSchema),
    defaultValues: { certificateChainPEM: "", privateKeyPEM: "", fqdn: "" },
  });

  const imported = useRef<SEPPCertificate | null>(null);

  const submit = async (values: ImportValues) => {
    if (!accessToken) return false;
    imported.current = await importSEPPCertificate(
      accessToken,
      values.certificateChainPEM,
      values.privateKeyPEM,
      values.fqdn.trim().toLowerCase() || undefined,
    );
  };

  const domain = mcc && mnc ? plmnDomain(mcc, mnc) : "";

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={() => {
        if (imported.current) onSuccess(imported.current);
      }}
      title="Import SEPP Certificate"
      description={`Replace ${PRODUCT.name}'s SEPP certificate with one issued outside ${PRODUCT.name}, for example by your CA. Its subjectAltName DNS name${domain ? ` under ${domain}` : ""} becomes the SEPP FQDN. The chain must end with the root or sub-CA certificate that partners import as the trust anchor.`}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to import SEPP certificate"
      submitLabel="Import"
      submittingLabel="Importing..."
      maxWidth="md"
    >
      <PemControl<ImportValues>
        name="certificateChainPEM"
        label="Certificate Chain"
        helperText="The certificate followed by the certificates that issued it, PEM."
        accept=".pem,.crt,.cer"
      />
      <PemControl<ImportValues>
        name="privateKeyPEM"
        label="Private Key"
        placeholder="-----BEGIN PRIVATE KEY-----"
        helperText="The certificate's private key, PEM: ECDSA P-256 or P-384, or RSA of at least 2048 bits. It is never shown again."
        accept=".pem,.key"
      />
      <TextControl<ImportValues>
        name="fqdn"
        label="FQDN"
        placeholder={domain ? `sepp1.${domain}` : ""}
        helperText="Optional. Only needed when the certificate has several DNS names under the home domain and its Common Name is not one of them: the name partners know this SEPP by."
      />
      <Alert severity="warning" sx={{ mt: 2 }}>
        If the new trust anchor differs from the current one, every roaming
        partner must import it before N32 with them works again.
      </Alert>
    </FormDialog>
  );
};

const generateSchema = (domain: string) =>
  yup.object({
    fqdn: yup
      .string()
      .default("")
      .test(
        "fqdn",
        "FQDN must be a fully qualified domain name",
        (v) => !v?.trim() || diameterIdentityRegex.test(v.trim()),
      )
      .test(
        "domain",
        `FQDN must be under ${domain}`,
        (v) =>
          !v?.trim() ||
          !domain ||
          v.trim().toLowerCase().endsWith(`.${domain}`),
      )
      .test(
        "host",
        `FQDN must be sepp<ID>.${domain || "<domain>"}, for example sepp1.${domain || "<domain>"}: the well-known sepp.${domain || "<domain>"} cannot name a SEPP`,
        (v) => {
          const fqdn = v?.trim().toLowerCase() ?? "";
          if (!fqdn || !domain || !fqdn.endsWith(`.${domain}`)) return true;
          const host = fqdn.slice(0, -domain.length - 1);
          return isGeneratedSEPPHost(host) && !isWellKnownSEPPHost(host);
        },
      ),
  });

type GenerateValues = yup.InferType<ReturnType<typeof generateSchema>>;

interface GenerateSEPPCertificateModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (cert: SEPPCertificate) => void;
  current?: SEPPCertificate | null;
  mcc?: string;
  mnc?: string;
}

export const GenerateSEPPCertificateModal: React.FC<
  GenerateSEPPCertificateModalProps
> = ({ open, onClose, onSuccess, current, mcc, mnc }) => {
  const { accessToken } = useAuth();
  const domain = mcc && mnc ? plmnDomain(mcc, mnc) : "";
  const currentInDomain =
    !!current &&
    !!domain &&
    current.fqdn.endsWith(`.${domain}`) &&
    isGeneratedSEPPHost(current.fqdn.slice(0, -domain.length - 1));
  const placeholder = currentInDomain
    ? current!.fqdn
    : mcc && mnc
      ? defaultSEPPFQDN(mcc, mnc)
      : "";

  const schema = useMemo(() => generateSchema(domain), [domain]);

  const form = useForm<GenerateValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { fqdn: "" },
  });

  const generated = useRef<SEPPCertificate | null>(null);

  const submit = async (values: GenerateValues) => {
    if (!accessToken) return false;
    generated.current = await generateSEPPCertificate(
      accessToken,
      values.fqdn.trim() || undefined,
    );
  };

  const verb = current ? "Regenerate" : "Generate";

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={() => {
        if (generated.current) onSuccess(generated.current);
      }}
      title={`${verb} SEPP Certificate`}
      description={`${PRODUCT.name} creates a new SEPP certificate signed by a new one-off CA. That CA is the new trust anchor to give partners. Generated certificates suit labs and bilateral pinning: they have no CRL distribution point and are not issued by an operator CA (TS 33.310 clause 6.1.3c.3).`}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to generate SEPP certificate"
      submitLabel={verb}
      submittingLabel="Generating..."
    >
      <TextControl<GenerateValues>
        name="fqdn"
        label="FQDN"
        placeholder={placeholder}
        helperText={`Optional. ${PRODUCT.name}'s SEPP FQDN, sepp<ID>${domain ? `.${domain}` : " under the home domain"}. Leave empty for ${placeholder || "the default"}.`}
        autoFocus
      />
      {current && (
        <Alert severity="warning" sx={{ mt: 2 }}>
          The trust anchor changes: every roaming partner must import the new
          one before N32 with them works again.
        </Alert>
      )}
    </FormDialog>
  );
};
