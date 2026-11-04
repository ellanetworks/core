// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useRef } from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  importSEPPCertificate,
  plmnDomain,
  seppFQDN,
  type SEPPCertificate,
} from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import PemControl from "@/components/form/PemControl";
import { diameterIdentityRegex } from "@/components/SMSCPeerModal";

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
  seppID: number;
  mcc?: string;
  mnc?: string;
}

const ImportSEPPCertificateModal: React.FC<ImportSEPPCertificateModalProps> = ({
  open,
  onClose,
  onSuccess,
  seppID,
  mcc,
  mnc,
}) => {
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
      seppID,
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
      title={`Import SEPP ${seppID} Certificate`}
      description={`Replace the certificate of SEPP ${seppID} with one issued by your CA${domain ? ` for a name under ${domain}` : ""}.`}
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
        helperText="The certificate, then its issuers, in PEM format."
        accept=".pem,.crt,.cer"
      />
      <PemControl<ImportValues>
        name="privateKeyPEM"
        label="Private Key"
        placeholder="-----BEGIN PRIVATE KEY-----"
        helperText="ECDSA P-256, P-384, or RSA 2048+, in PEM format."
        accept=".pem,.key"
      />
      <TextControl<ImportValues>
        name="fqdn"
        label="FQDN"
        placeholder={mcc && mnc ? seppFQDN(mcc, mnc, seppID) : ""}
        helperText="Optional. Needed only if the certificate has several DNS names."
      />
    </FormDialog>
  );
};

export default ImportSEPPCertificateModal;
