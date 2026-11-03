// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  ContentCopy as CopyIcon,
  Download as DownloadIcon,
} from "@mui/icons-material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import ListPageHeader from "@/components/ListPageHeader";
import {
  GenerateSEPPCertificateModal,
  ImportSEPPCertificateModal,
} from "@/components/SEPPCertificateModals";
import {
  getSEPPCertificate,
  plmnDomain,
  type SEPPCertificate,
} from "@/queries/roaming";
import { getOperator, type OperatorData } from "@/queries/operator";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { useNow } from "@/hooks/useNow";
import { formatDateTime } from "@/utils/formatters";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { PRODUCT } from "@/utils/product";
import { useRoamingContext } from "./types";

const DAY_MS = 24 * 60 * 60 * 1000;
export const EXPIRY_WARNING_DAYS = 30;

const downloadText = (filename: string, text: string) => {
  const url = window.URL.createObjectURL(
    new Blob([text], { type: "application/x-pem-file" }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

const settingCellSx = { fontWeight: 600, width: "30%" } as const;

const ExpiryChip = ({ notAfter, now }: { notAfter: string; now: number }) => {
  const daysLeft = Math.floor((new Date(notAfter).getTime() - now) / DAY_MS);
  if (daysLeft < 0) {
    return <Chip size="small" color="error" label="Expired" />;
  }
  if (daysLeft <= EXPIRY_WARNING_DAYS) {
    return (
      <Chip
        size="small"
        color="warning"
        label={
          daysLeft === 1 ? "Expires in 1 day" : `Expires in ${daysLeft} days`
        }
      />
    );
  }
  return null;
};

export default function SEPPTab() {
  const { accessToken, canEdit, showSnackbar } = useRoamingContext();
  const queryClient = useQueryClient();
  const copy = useCopyToClipboard();
  const now = useNow(60_000);

  const [dialog, setDialog] = useState<"import" | "generate" | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);

  const certQuery = useQuery<SEPPCertificate | null>({
    queryKey: ["sepp-certificate"],
    queryFn: () => getSEPPCertificate(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const operatorQuery = useQuery<OperatorData>({
    queryKey: ["operator"],
    queryFn: () => getOperator(accessToken || ""),
    enabled: !!accessToken,
  });

  const mcc = operatorQuery.data?.id.mcc;
  const mnc = operatorQuery.data?.id.mnc;
  const domain = mcc && mnc ? plmnDomain(mcc, mnc) : "";

  const handleReplaced = (cert: SEPPCertificate, action: string) => {
    queryClient.setQueryData(["sepp-certificate"], cert);
    void queryClient.invalidateQueries({ queryKey: ["sepp-certificate"] });
    setWarnings(cert.warnings ?? []);
    showSnackbar(`SEPP certificate ${action} successfully.`, "success");
  };

  const actions = (current: SEPPCertificate | null) =>
    canEdit && (
      <Stack direction="row" spacing={1}>
        <Button variant="outlined" onClick={() => setDialog("import")}>
          Import
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={() => setDialog("generate")}
        >
          {current ? "Regenerate" : "Generate"}
        </Button>
      </Stack>
    );

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <ListPageHeader
        title="SEPP"
        description={`${PRODUCT.name}'s SEPP certificate, used for N32 with the SEPPs of 5G roaming partners. Give partners its trust anchor.`}
        variant="section"
        action={certQuery.data !== undefined && actions(certQuery.data)}
      />

      {warnings
        .filter(
          (warning) => !(certQuery.data?.warnings ?? []).includes(warning),
        )
        .map((warning) => (
          <Alert
            key={warning}
            severity="warning"
            sx={{ mb: 2 }}
            onClose={() =>
              setWarnings((prev) => prev.filter((w) => w !== warning))
            }
          >
            {warning}
          </Alert>
        ))}

      <QueryState
        query={certQuery}
        resource="SEPP certificate"
        isEmpty={(data) => data === null}
        empty={
          <EmptyState
            primaryText="No SEPP certificate yet"
            secondaryText={
              canEdit
                ? `${PRODUCT.name} generates one when you add the first partner with 5G roaming. You can also generate or import one now.`
                : `${PRODUCT.name} generates one when the first partner with 5G roaming is added.`
            }
          />
        }
      >
        {(cert) =>
          cert && (
            <>
              {domain && !cert.fqdn.endsWith(`.${domain}`) && (
                <Alert severity="error" sx={{ mb: 2 }}>
                  The SEPP FQDN {cert.fqdn} is not under {domain}, the domain of
                  the operator PLMN. Partners will reject it: generate or import
                  a certificate for this PLMN.
                </Alert>
              )}
              {(cert.warnings ?? []).map((warning) => (
                <Alert key={warning} severity="warning" sx={{ mb: 2 }}>
                  {warning}
                </Alert>
              ))}
              {now > 0 && new Date(cert.notAfter).getTime() < now && (
                <Alert severity="error" sx={{ mb: 2 }}>
                  The SEPP certificate expired on{" "}
                  {formatDateTime(cert.notAfter)}. Partners reject it: generate
                  or import a new one.
                </Alert>
              )}
              <TableContainer sx={TABLE_CONTAINER_SX}>
                <Table>
                  <TableBody>
                    <TableRow>
                      <TableCell sx={settingCellSx}>
                        <Tooltip
                          title="The certificate's subjectAltName DNS name, the name partners know this SEPP by"
                          arrow
                        >
                          <span>FQDN</span>
                        </Tooltip>
                      </TableCell>
                      <TableCell sx={{ wordBreak: "break-all" }}>
                        {cert.fqdn}
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell sx={settingCellSx}>
                        <Tooltip
                          title="SHA-256 fingerprint of the certificate"
                          arrow
                        >
                          <span>Fingerprint</span>
                        </Tooltip>
                      </TableCell>
                      <TableCell>
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 0.5,
                          }}
                        >
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: "monospace",
                              wordBreak: "break-all",
                            }}
                          >
                            {cert.fingerprint}
                          </Typography>
                          <Tooltip title="Copy fingerprint" arrow>
                            <IconButton
                              size="small"
                              onClick={() =>
                                copy(cert.fingerprint, "Fingerprint")
                              }
                              aria-label="Copy fingerprint"
                            >
                              <CopyIcon fontSize="small" color="primary" />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell sx={settingCellSx}>Expires</TableCell>
                      <TableCell>
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1,
                          }}
                        >
                          <span>{formatDateTime(cert.notAfter)}</span>
                          {now > 0 && (
                            <ExpiryChip notAfter={cert.notAfter} now={now} />
                          )}
                        </Box>
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell sx={settingCellSx}>
                        <Tooltip
                          title="The root or sub-CA certificate partners configure to trust this SEPP"
                          arrow
                        >
                          <span>Trust Anchor</span>
                        </Tooltip>
                      </TableCell>
                      <TableCell>
                        <Stack direction="row" spacing={1}>
                          <Button
                            size="small"
                            startIcon={<DownloadIcon />}
                            onClick={() =>
                              downloadText(
                                `${cert.fqdn}-trust-anchor.pem`,
                                cert.trustAnchorPEM,
                              )
                            }
                          >
                            Download
                          </Button>
                          <Button
                            size="small"
                            startIcon={<CopyIcon />}
                            onClick={() =>
                              copy(cert.trustAnchorPEM, "Trust anchor")
                            }
                          >
                            Copy
                          </Button>
                        </Stack>
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell sx={settingCellSx}>
                        <Tooltip
                          title="The SEPP certificate followed by its chain"
                          arrow
                        >
                          <span>Certificate Chain</span>
                        </Tooltip>
                      </TableCell>
                      <TableCell>
                        <Button
                          size="small"
                          startIcon={<DownloadIcon />}
                          onClick={() =>
                            downloadText(
                              `${cert.fqdn}-chain.pem`,
                              cert.certificateChainPEM,
                            )
                          }
                        >
                          Download
                        </Button>
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )
        }
      </QueryState>

      {dialog === "import" && (
        <ImportSEPPCertificateModal
          open
          onClose={() => setDialog(null)}
          onSuccess={(cert) => handleReplaced(cert, "imported")}
          mcc={mcc}
          mnc={mnc}
        />
      )}
      {dialog === "generate" && (
        <GenerateSEPPCertificateModal
          open
          onClose={() => setDialog(null)}
          onSuccess={(cert) => handleReplaced(cert, "generated")}
          current={certQuery.data}
          mcc={mcc}
          mnc={mnc}
        />
      )}
    </Box>
  );
}
