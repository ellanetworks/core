// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch } from "@/queries/utils";
import type { APIUser, RoleID } from "@/queries/users";
import { PRODUCT } from "@/utils/product";

export interface GroupRole {
  group: string;
  role_id: RoleID;
}

export interface OIDCSettings {
  enabled: boolean;
  external_url: string;
  redirect_url: string;
  post_logout_redirect_url: string;
  issuer: string;
  client_id: string;
  client_secret_set: boolean;
  display_name: string;
  scopes: string[];
  session_lifetime_seconds: number;
  trust_unverified_email: boolean;
  ca_certificate: string;
  groups_claim: string;
  group_roles: GroupRole[];
}

export interface UpdateOIDCSettings {
  enabled: boolean;
  external_url: string;
  issuer: string;
  client_id: string;
  client_secret: string;
  display_name: string;
  scopes: string[];
  session_lifetime_seconds: number;
  trust_unverified_email: boolean;
  ca_certificate: string;
  groups_claim: string;
  group_roles: GroupRole[];
}

export interface OIDCDiscovery {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  jwks_uri: string;
  signing_algorithms: string[];
  keys: number;
  pkce: boolean;
  issuer_parameter: boolean;
  token_endpoint_auth_method: "client_secret_basic" | "client_secret_post";
  userinfo_endpoint: string;
}

export interface OIDCLogin {
  enabled: boolean;
  display_name?: string;
}

export const OIDC_QUERY_KEY = ["oidc"] as const;

export const MIN_SESSION_LIFETIME_SECONDS = 300;
export const MAX_SESSION_LIFETIME_SECONDS = 30 * 24 * 3600;
export const MAX_SCOPES = 20;
export const MAX_GROUP_ROLES = 100;
export const MAX_DISPLAY_NAME_BYTES = 64;
export const MAX_VALUE_BYTES = 255;
export const MAX_URL_BYTES = 2048;
export const MAX_CA_CERTIFICATE_BYTES = 64 * 1024;
export const REQUIRED_SCOPES = ["openid", "email"];
export const OIDC_LOGIN_PATH = "/api/v1/auth/oidc/login";
export const OIDC_LINK_PATH = `${OIDC_LOGIN_PATH}?link=1`;
export const OIDC_CALLBACK_PATH = "/api/v1/auth/oidc/callback";
export const OIDC_POST_LOGOUT_PATH = "/login";

export const getOIDC = (authToken: string) =>
  apiFetch<OIDCSettings>("/api/v1/oidc", { authToken });

export const updateOIDC = (authToken: string, settings: UpdateOIDCSettings) =>
  apiFetch<OIDCSettings>("/api/v1/oidc", {
    method: "PUT",
    authToken,
    body: settings,
  });

export const discoverOIDC = (
  authToken: string,
  issuer: string,
  caCertificate: string,
) =>
  apiFetch<OIDCDiscovery>("/api/v1/oidc/discovery", {
    method: "POST",
    authToken,
    body: { issuer, ca_certificate: caCertificate },
  });

export const getOIDCLogin = () => apiFetch<OIDCLogin>("/api/v1/auth/oidc");

export const currentUpdate = (settings: OIDCSettings): UpdateOIDCSettings => ({
  enabled: settings.enabled,
  external_url: settings.external_url,
  issuer: settings.issuer,
  client_id: settings.client_id,
  client_secret: "",
  display_name: settings.display_name,
  scopes: settings.scopes,
  session_lifetime_seconds: settings.session_lifetime_seconds,
  trust_unverified_email: settings.trust_unverified_email,
  ca_certificate: settings.ca_certificate,
  groups_claim: settings.groups_claim,
  group_roles: settings.group_roles,
});

export const roleFromGroups = (user: APIUser, settings?: OIDCSettings) =>
  settings?.enabled === true &&
  settings.group_roles.length > 0 &&
  user.oidc_linked;

const oidcLoginErrors: Record<string, string> = {
  rate_limited: "Too many login attempts. Try again in a minute.",
  unavailable:
    "Single sign-on is not available. Try again later, or log in with your password.",
  failed: "Single sign-on failed. Try again.",
  denied: "Your identity provider did not let you sign in.",
  not_set_up: `Your account has not been set up in ${PRODUCT.name}. Contact an administrator.`,
  email_not_verified:
    "Your identity provider has not verified your email address. Contact an administrator.",
  linked_elsewhere: `Your ${PRODUCT.name} account is linked to another identity provider account. Contact an administrator.`,
  no_group: `None of your groups gives access to ${PRODUCT.name}. Contact an administrator.`,
  groups_overage:
    "You are in too many groups for your identity provider to list them. Contact an administrator.",
  too_many_users: `${PRODUCT.name} has reached its number of users. Contact an administrator.`,
  error: "Single sign-on failed. Try again, or contact an administrator.",
};

export const oidcLoginError = (code: string, displayName?: string) => {
  if (code === "link_required") {
    return `This account has a password. Log in with it, then link your ${displayName || "identity provider"} account from your profile.`;
  }
  return Object.hasOwn(oidcLoginErrors, code)
    ? oidcLoginErrors[code]
    : oidcLoginErrors.error;
};

export const oidcLinkError = (code: string) =>
  code === "linked_elsewhere"
    ? "That account at your identity provider, or yours, is already linked."
    : oidcLoginError(code);

const LINKING_KEY = "core.oidcLinking";

export function rememberLinking() {
  try {
    sessionStorage.setItem(LINKING_KEY, "1");
  } catch {
    return;
  }
}

export function isLinking(): boolean {
  try {
    return sessionStorage.getItem(LINKING_KEY) === "1";
  } catch {
    return false;
  }
}

export function forgetLinking() {
  try {
    sessionStorage.removeItem(LINKING_KEY);
  } catch {
    return;
  }
}

const byteLength = (value: string) => new TextEncoder().encode(value).length;

const CONTROL = /\p{Cc}/u;

export function textError(
  value: string,
  maxBytes: number,
  missing?: string,
): string | undefined {
  if (value === "") return missing;
  if (byteLength(value) > maxBytes) return `At most ${maxBytes} bytes.`;
  if (CONTROL.test(value)) return "No control characters.";
  return undefined;
}

export function httpsURLError(
  value: string,
  { origin = false, missing }: { origin?: boolean; missing?: string } = {},
): string | undefined {
  if (value === "") return missing;
  if (byteLength(value) > MAX_URL_BYTES)
    return `At most ${MAX_URL_BYTES} bytes.`;
  if (!/^https:\/\//i.test(value)) return "An https URL.";
  if (/[?#]/.test(value)) return "With no query or fragment.";
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return "An https URL.";
  }
  if (/[\s\\]|\p{Cc}/u.test(value) || url.hostname === "") {
    return "An https URL.";
  }
  if (/^https:\/\/[^/]*@/i.test(value)) return "With no user name or password.";
  if (origin && !/^https:\/\/[^/]+\/?$/i.test(value)) return "With no path.";
  return undefined;
}

const SCOPE = /^[\x21\x23-\x5b\x5d-\x7e]+$/;

export const splitScopes = (value: string) =>
  value.split(/\s+/).filter(Boolean);

export function scopesError(scopes: string[]): string | undefined {
  if (!REQUIRED_SCOPES.every((s) => scopes.includes(s))) {
    return `Include ${REQUIRED_SCOPES.join(" and ")}.`;
  }
  if (scopes.length > MAX_SCOPES) return `At most ${MAX_SCOPES} scopes.`;
  for (const [i, scope] of scopes.entries()) {
    if (!SCOPE.test(scope) || scope.length > MAX_VALUE_BYTES) {
      return `${scope} is not a valid scope.`;
    }
    if (scopes.indexOf(scope) !== i) return `${scope} appears more than once.`;
  }
  return undefined;
}

export function caCertificateError(value: string): string | undefined {
  if (value === "") return undefined;
  if (byteLength(value) > MAX_CA_CERTIFICATE_BYTES) return "At most 64 KiB.";
  if (!value.includes("-----BEGIN CERTIFICATE-----")) {
    return "PEM certificates, each starting with -----BEGIN CERTIFICATE-----.";
  }
  return undefined;
}

export function groupErrors(groups: string[]): (string | undefined)[] {
  return groups.map(
    (group, i) =>
      textError(group, MAX_VALUE_BYTES, "Enter the group.") ??
      (groups.indexOf(group) !== i ? "Appears more than once." : undefined),
  );
}
