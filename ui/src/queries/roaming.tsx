// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export const DEFAULT_ROAMING_DIAMETER_PORT = 3868;

export type RoamingPartnerSEPP = {
  fqdn: string;
  n32cURI: string;
  n32fURI?: string;
  trustAnchorPEM: string;
};

export type PLMNID = {
  mcc: string;
  mnc: string;
};

export type RoamingPartnerPLMN = PLMNID & {
  nrfAPIRoot?: string;
};

export type RoamingPartnerInput = {
  name: string;
  plmns: RoamingPartnerPLMN[];
  profileName: string;
  sepp?: RoamingPartnerSEPP;
  diameterPeerID?: string;
};

export type N32State =
  "idle" | "establishing" | "established" | "failed" | "unknown";

export type RoamingPartnerN32 = {
  state: N32State;
  peer?: string;
  lastError?: string;
};

export type RoamingPartner = RoamingPartnerInput & {
  id: string;
  n32?: RoamingPartnerN32;
};

export type RoamingDiameterPeerInput = {
  diameterIdentity: string;
  address: string;
  port: number;
};

export type RoamingDiameterPeer = RoamingDiameterPeerInput & {
  id: string;
};

export type SEPPSource = "generated" | "imported";

export type SEPPCertificate = {
  seppID: number;
  fqdn: string;
  source: SEPPSource;
  subject?: string;
  sans?: string[];
  issuer?: string;
  notBefore?: string;
  notAfter?: string;
  fingerprint?: string;
  trustAnchorFingerprint?: string;
  warnings?: string[];
};

export type SEPPTrustAnchor = {
  source: SEPPSource;
  subject: string;
  fingerprint: string;
  notAfter: string;
  pem: string;
  seppIDs: number[];
};

export type SEPPCARegenerated = {
  trustAnchor: SEPPTrustAnchor;
  warnings?: string[];
};

export type SEPPListener = {
  seppID?: number;
  configured: boolean;
  address?: string;
  port?: number;
  listening: boolean;
  error?: string;
};

export type SEPPStatus = {
  listener: SEPPListener;
  certificates: SEPPCertificate[];
  trustAnchors: SEPPTrustAnchor[];
};

const ROAMING_PATH = "/api/v1/operator/roaming";
const PARTNERS_PATH = `${ROAMING_PATH}/partners`;
const PEERS_PATH = `${ROAMING_PATH}/diameter-peers`;
const SEPP_PATH = `${ROAMING_PATH}/sepp`;

const seppCertificatePath = (seppID: number) =>
  `${SEPP_PATH}/nodes/${seppID}/certificate`;

export const listRoamingPartners = async (
  authToken: string,
): Promise<RoamingPartner[]> => {
  const res = await apiFetch<{ items: RoamingPartner[] }>(PARTNERS_PATH, {
    authToken,
  });
  return res?.items ?? [];
};

export const createRoamingPartner = async (
  authToken: string,
  partner: RoamingPartnerInput,
): Promise<RoamingPartner> =>
  apiFetch<RoamingPartner>(PARTNERS_PATH, {
    method: "POST",
    authToken,
    body: partner,
  });

export const updateRoamingPartner = async (
  authToken: string,
  id: string,
  partner: RoamingPartnerInput,
): Promise<void> =>
  apiFetchVoid(`${PARTNERS_PATH}/${encodeURIComponent(id)}`, {
    method: "PUT",
    authToken,
    body: partner,
  });

export const deleteRoamingPartner = async (
  authToken: string,
  id: string,
): Promise<void> =>
  apiFetchVoid(`${PARTNERS_PATH}/${encodeURIComponent(id)}`, {
    method: "DELETE",
    authToken,
  });

export const rehandshakeRoamingPartner = async (
  authToken: string,
  id: string,
): Promise<RoamingPartnerN32 | undefined> => {
  const res = await apiFetch<{ n32: RoamingPartnerN32 }>(
    `${PARTNERS_PATH}/${encodeURIComponent(id)}/handshake`,
    { method: "POST", authToken },
  );
  return res?.n32;
};

export const getSEPPStatus = async (authToken: string): Promise<SEPPStatus> =>
  apiFetch<SEPPStatus>(SEPP_PATH, { authToken });

export const listRoamingDiameterPeers = async (
  authToken: string,
): Promise<RoamingDiameterPeer[]> => {
  const res = await apiFetch<{ items: RoamingDiameterPeer[] }>(PEERS_PATH, {
    authToken,
  });
  return res?.items ?? [];
};

export const createRoamingDiameterPeer = async (
  authToken: string,
  peer: RoamingDiameterPeerInput,
): Promise<RoamingDiameterPeer> =>
  apiFetch<RoamingDiameterPeer>(PEERS_PATH, {
    method: "POST",
    authToken,
    body: peer,
  });

export const updateRoamingDiameterPeer = async (
  authToken: string,
  id: string,
  peer: RoamingDiameterPeerInput,
): Promise<void> =>
  apiFetchVoid(`${PEERS_PATH}/${encodeURIComponent(id)}`, {
    method: "PUT",
    authToken,
    body: peer,
  });

export const deleteRoamingDiameterPeer = async (
  authToken: string,
  id: string,
): Promise<void> =>
  apiFetchVoid(`${PEERS_PATH}/${encodeURIComponent(id)}`, {
    method: "DELETE",
    authToken,
  });

export const importSEPPCertificate = async (
  authToken: string,
  seppID: number,
  certificateChainPEM: string,
  privateKeyPEM: string,
  fqdn?: string,
): Promise<SEPPCertificate> =>
  apiFetch<SEPPCertificate>(seppCertificatePath(seppID), {
    method: "PUT",
    authToken,
    body: fqdn
      ? { certificateChainPEM, privateKeyPEM, fqdn }
      : { certificateChainPEM, privateKeyPEM },
  });

export const deleteSEPPCertificate = async (
  authToken: string,
  seppID: number,
): Promise<void> =>
  apiFetchVoid(seppCertificatePath(seppID), {
    method: "DELETE",
    authToken,
  });

export const regenerateSEPPCA = async (
  authToken: string,
): Promise<SEPPCARegenerated> =>
  apiFetch<SEPPCARegenerated>(`${SEPP_PATH}/ca/regenerate`, {
    method: "POST",
    authToken,
  });

const paddedMNC = (mnc: string) => (mnc.length === 2 ? `0${mnc}` : mnc);

export const plmnDomain = (mcc: string, mnc: string) =>
  `5gc.mnc${paddedMNC(mnc)}.mcc${mcc}.3gppnetwork.org`;

export const defaultNRFAPIRoot = (mcc: string, mnc: string) =>
  `https://nrf.${plmnDomain(mcc, mnc)}`;

export const seppFQDN = (mcc: string, mnc: string, seppID: number) =>
  `sepp${seppID}.${plmnDomain(mcc, mnc)}`;

export const formatPLMN = (plmn: PLMNID) => `${plmn.mcc}-${plmn.mnc}`;

export const samePLMN = (a: PLMNID, b: PLMNID) =>
  a.mcc === b.mcc && paddedMNC(a.mnc) === paddedMNC(b.mnc);

const DNS_LABEL_RE = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

export type SEPPFQDNParts = {
  host: string;
  plmn: PLMNID;
};

export const parseSEPPFQDN = (fqdn: string): SEPPFQDNParts | null => {
  const labels = fqdn.toLowerCase().replace(/\.$/, "").split(".");
  let tail: number;
  if (labels.at(-2) === "3gppnetwork" && labels.at(-1) === "org") {
    tail = labels.length - 2;
  } else if (labels.at(-2) === "ipxnetwork" && labels.at(-1) === "org") {
    tail = labels.length - 3;
  } else {
    return null;
  }
  const mcc = labels[tail - 1]?.match(/^mcc(\d{3})$/)?.[1];
  const mnc = labels[tail - 2]?.match(/^mnc(\d{3})$/)?.[1];
  if (!mcc || !mnc || labels[tail - 3] !== "5gc") return null;
  const hostLabels = labels.slice(0, tail - 3);
  if (
    hostLabels.length === 0 ||
    !labels.every((label) => DNS_LABEL_RE.test(label))
  ) {
    return null;
  }
  return { host: hostLabels.join("."), plmn: { mcc, mnc } };
};

export const isWellKnownSEPPHost = (host: string) => host === "sepp";

export const formatEndpoint = (address: string, port: number) =>
  address.includes(":") ? `[${address}]:${port}` : `${address}:${port}`;

export const diameterPeerLabel = (peer: RoamingDiameterPeer) => {
  const endpoint = formatEndpoint(peer.address, peer.port);
  return peer.diameterIdentity
    ? `${peer.diameterIdentity} (${endpoint})`
    : endpoint;
};
