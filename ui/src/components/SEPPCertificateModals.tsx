// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useRef } from "react";
import { Alert } from "@mui/material";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  generateSEPPCertificate,
  importSEPPCertificate,
  plmnDomain,
  regenerateSEPPCA,
  seppFQDN,
  type SEPPCARegenerated,
  type SEPPCertificate,
} from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import ConfirmDialog from "@/components/form/ConfirmDialog";
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
  seppID: number;
  mcc?: string;
  mnc?: string;
}

export const ImportSEPPCertificateModal: React.FC<
  ImportSEPPCertificateModalProps
> = ({ open, onClose, onSuccess, seppID, mcc, mnc }) => {
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
      description={`Replace the certificate of SEPP ${seppID} with one issued outside ${PRODUCT.name}, for example by your CA. Its subjectAltName DNS name${domain ? ` under ${domain}` : ""} becomes the FQDN of SEPP ${seppID}. The chain must end with the root or sub-CA certificate that partners import as the trust anchor.`}
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
        placeholder={mcc && mnc ? seppFQDN(mcc, mnc, seppID) : ""}
        helperText="Optional. Only needed when the certificate has several DNS names under the home domain and its Common Name is not one of them: the name partners know this SEPP by. No two nodes may have the same FQDN."
      />
      <Alert severity="warning" sx={{ mt: 2 }}>
        If the new trust anchor differs from the current one, every roaming
        partner must import it before N32 with this SEPP works again.
      </Alert>
    </FormDialog>
  );
};

interface GenerateSEPPCertificateModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (cert: SEPPCertificate) => void;
  seppID: number;
  imported: boolean;
  mcc?: string;
  mnc?: string;
}

export const GenerateSEPPCertificateModal: React.FC<
  GenerateSEPPCertificateModalProps
> = ({ open, onClose, onSuccess, seppID, imported, mcc, mnc }) => {
  const { accessToken } = useAuth();
  const fqdn = mcc && mnc ? seppFQDN(mcc, mnc, seppID) : `sepp${seppID}`;

  const confirm = async () => {
    if (!accessToken) return;
    onSuccess(await generateSEPPCertificate(accessToken, seppID));
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={confirm}
      title={`Generate SEPP ${seppID} Certificate`}
      description={`${PRODUCT.name} replaces the certificate of SEPP ${seppID} with a new one for ${fqdn}, signed by the cluster SEPP CA. Generated certificates suit labs and bilateral agreements: they have no CRL distribution point and are not issued by an operator CA (TS 33.310 clause 6.1.3c.3).`}
      extra={
        imported && (
          <Alert severity="warning" sx={{ mt: 2 }}>
            The current certificate is imported: the trust anchor of SEPP{" "}
            {seppID} becomes the cluster SEPP CA, which every roaming partner
            must import before N32 with this SEPP works again.
          </Alert>
        )
      }
      confirmLabel="Generate"
      confirmingLabel="Generating..."
      confirmColor="primary"
    />
  );
};

interface RegenerateSEPPCAModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (result: SEPPCARegenerated) => void;
}

export const RegenerateSEPPCAModal: React.FC<RegenerateSEPPCAModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const confirm = async () => {
    if (!accessToken) return;
    onSuccess(await regenerateSEPPCA(accessToken));
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={confirm}
      title="Regenerate SEPP CA"
      description={`${PRODUCT.name} replaces the cluster SEPP CA and reissues every generated SEPP certificate with the new CA. Imported certificates are kept.`}
      extra={
        <Alert severity="warning" sx={{ mt: 2 }}>
          The trust anchor of every SEPP with a generated certificate changes:
          every roaming partner must import the new one before N32 with them
          works again.
        </Alert>
      }
      confirmLabel="Regenerate"
      confirmingLabel="Regenerating..."
    />
  );
};
