// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { ApiError, apiFetch, apiFetchVoid } from "@/queries/utils";
import { formatDateTime } from "@/utils/formatters";

export const DEFAULT_ROAMING_DIAMETER_PORT = 3868;

export type RoamingPartnerSEPP = {
  fqdn: string;
  n32cURI: string;
  n32fURI?: string;
  trustAnchorPEM: string;
  nrfAPIRoot?: string;
};

export type PLMNID = {
  mcc: string;
  mnc: string;
};

export type RoamingPartnerInput = {
  name: string;
  plmns: PLMNID[];
  profileName: string;
  sepp?: RoamingPartnerSEPP;
  diameterPeerID?: string;
};

export type N32State =
  "idle" | "establishing" | "established" | "failed" | "terminated" | "unknown";

export type RoamingPartnerN32 = {
  nodeID?: string;
  seppID?: number;
  state: N32State;
  since?: string;
  initiator?: "local" | "peer";
  nftlst: boolean;
  peer?: string;
  lastError?: string;
  otherPeers?: RoamingPartnerN32Peer[];
};

export type RoamingPartnerN32Peer = {
  peer: string;
  since: string;
  nftlst: boolean;
};

export type RoamingPartner = RoamingPartnerInput & {
  id: string;
  n32?: RoamingPartnerN32[];
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
  certificateChainPEM: string;
  trustAnchorPEM: string;
  fingerprint: string;
  notAfter: string;
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

export type SEPPCertificateSummary = {
  subject: string;
  sans: string[];
  issuer: string;
  notBefore: string;
  notAfter: string;
  fingerprint: string;
  trustAnchorFingerprint: string;
};

export type SEPPNodeStatus = {
  nodeID?: string;
  seppID?: number;
  fqdn?: string;
  source?: SEPPSource;
  reachable: boolean;
  n32Configured: boolean;
  address?: string;
  port?: number;
  listening: boolean;
  certificate?: SEPPCertificateSummary;
  error?: string;
};

export type SEPPStatus = {
  trustAnchors: SEPPTrustAnchor[];
  nodes: SEPPNodeStatus[];
};

export type SEPPCARegenerated = {
  trustAnchor: SEPPTrustAnchor;
  reissued: number[];
  warnings?: string[];
};

export type RoamingUsage = {
  date: string;
  mcc: string;
  mnc: string;
  partner_name?: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

export type RoamerRAT = "4G" | "5G";

export type RoamerRegistration = {
  rat: RoamerRAT;
  nodeID: string;
  reachable: boolean;
  registered?: boolean;
  connectionState?: "connected" | "idle";
  since?: string;
  lastSeenAt?: string;
  radio?: string;
};

export type RoamerSession = {
  rat: RoamerRAT;
  nodeID: string;
  id: number;
  dataNetwork: string;
  ipv4Address?: string;
  ipv6Prefix?: string;
};

export type RoamerUsage = {
  uplinkBytes: number;
  downlinkBytes: number;
  totalBytes: number;
};

export type Roamer = {
  imsi: string;
  partnerID: string;
  partnerName: string;
  homePLMN: PLMNID;
  registrations: RoamerRegistration[];
  sessions: RoamerSession[];
  usageToday?: RoamerUsage;
};

export type UnreachableNode = {
  nodeID: string;
  error: string;
};

export type RoamersList = {
  items: Roamer[];
  unreachableNodes?: UnreachableNode[];
};

const PARTNERS_PATH = "/api/v1/roaming/partners";
const PEERS_PATH = "/api/v1/roaming/diameter-peers";
const SEPP_PATH = "/api/v1/roaming/sepp";
const ROAMERS_PATH = "/api/v1/roaming/roamers";

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
): Promise<RoamingPartnerN32[]> => {
  const res = await apiFetch<{ n32: RoamingPartnerN32[] }>(
    `${PARTNERS_PATH}/${encodeURIComponent(id)}/handshake`,
    { method: "POST", authToken },
  );
  return res?.n32 ?? [];
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

export const getSEPPCertificate = async (
  authToken: string,
  seppID: number,
): Promise<SEPPCertificate | null> => {
  try {
    return await apiFetch<SEPPCertificate>(seppCertificatePath(seppID), {
      authToken,
    });
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
};

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

export const generateSEPPCertificate = async (
  authToken: string,
  seppID: number,
): Promise<SEPPCertificate> =>
  apiFetch<SEPPCertificate>(`${seppCertificatePath(seppID)}/generate`, {
    method: "POST",
    authToken,
  });

export const regenerateSEPPCA = async (
  authToken: string,
): Promise<SEPPCARegenerated> =>
  apiFetch<SEPPCARegenerated>(`${SEPP_PATH}/ca/regenerate`, {
    method: "POST",
    authToken,
  });

export const getRoamingUsage = async (
  authToken: string,
  start: string,
  end: string,
  imsi: string,
): Promise<RoamingUsage[]> => {
  const params = new URLSearchParams();
  if (start) params.set("start", start);
  if (end) params.set("end", end);
  if (imsi) params.set("imsi", imsi);
  const query = params.toString();
  const res = await apiFetch<RoamingUsage[]>(
    `/api/v1/roaming/usage${query ? `?${query}` : ""}`,
    { authToken },
  );
  return res ?? [];
};

export const listRoamers = async (authToken: string): Promise<RoamersList> => {
  const res = await apiFetch<RoamersList>(ROAMERS_PATH, { authToken });
  return {
    items: res?.items ?? [],
    unreachableNodes: res?.unreachableNodes ?? [],
  };
};

export const getRoamer = async (
  authToken: string,
  imsi: string,
): Promise<Roamer> =>
  apiFetch<Roamer>(`${ROAMERS_PATH}/${encodeURIComponent(imsi)}`, {
    authToken,
  });

export const deregisterRoamer = async (
  authToken: string,
  imsi: string,
): Promise<void> =>
  apiFetchVoid(`${ROAMERS_PATH}/${encodeURIComponent(imsi)}`, {
    method: "DELETE",
    authToken,
  });

const paddedMNC = (mnc: string) => (mnc.length === 2 ? `0${mnc}` : mnc);

export const plmnDomain = (mcc: string, mnc: string) =>
  `5gc.mnc${paddedMNC(mnc)}.mcc${mcc}.3gppnetwork.org`;

export const defaultNRFAPIRoot = (mcc: string, mnc: string) =>
  `https://nrf.${plmnDomain(mcc, mnc)}`;

export const seppFQDN = (mcc: string, mnc: string, seppID: number) =>
  `sepp${seppID}.${plmnDomain(mcc, mnc)}`;

export const n32NodeLabel = (n32: { nodeID?: string; seppID?: number }) => {
  if (n32.seppID) return `SEPP ${n32.seppID}`;
  if (n32.nodeID) return `Node ${n32.nodeID}`;
  return "SEPP";
};

export const otherPeerLabel = (peer: RoamingPartnerN32Peer) =>
  peer.since ? `${peer.peer} since ${formatDateTime(peer.since)}` : peer.peer;

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
