// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export const roleIDToLabel = (role: RoleID): string => {
  switch (role) {
    case RoleID.Admin:
      return "Admin";
    case RoleID.NetworkManager:
      return "Network Manager";
    case RoleID.ReadOnly:
      return "Read Only";
    default:
      return "Unknown";
  }
};

export enum RoleID {
  Admin = 1,
  ReadOnly = 2,
  NetworkManager = 3,
}

export type APIUser = {
  email: string;
  role_id: RoleID;
  role_pinned: boolean;
  password_set: boolean;
  oidc_linked: boolean;
};

export type ListUsersResponse = {
  items: APIUser[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listUsers(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListUsersResponse> {
  return apiFetch<ListUsersResponse>(
    `/api/v1/users?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export const getLoggedInUser = async (authToken: string): Promise<APIUser> => {
  return apiFetch<APIUser>(`/api/v1/users/me`, { authToken });
};

export const createUser = async (
  authToken: string,
  email: string,
  role_id: RoleID,
  password: string,
  role_pinned?: boolean,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users`, {
    method: "POST",
    authToken,
    body: { email, password, role_id, role_pinned },
  });
};

export const updateUserPassword = async (
  authToken: string,
  email: string,
  password: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users/${email}/password`, {
    method: "PUT",
    authToken,
    body: { password },
  });
};

export const updateMyUserPassword = async (
  authToken: string,
  currentPassword: string,
  password: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users/me/password`, {
    method: "PUT",
    authToken,
    body: { current_password: currentPassword, password },
  });
};

export const updateUser = async (
  authToken: string,
  email: string,
  role_id: RoleID,
  role_pinned?: boolean,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users/${email}`, {
    method: "PUT",
    authToken,
    body: { role_id, role_pinned },
  });
};

export const removeUserPassword = async (
  authToken: string,
  email: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users/${email}/password`, {
    method: "DELETE",
    authToken,
  });
};

export const unlinkUser = async (
  authToken: string,
  email: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users/${email}/oidc`, {
    method: "DELETE",
    authToken,
  });
};

export const signInMethods = (user: APIUser): string[] => {
  const methods = [];
  if (user.password_set) methods.push("Password");
  if (user.oidc_linked) methods.push("Single sign-on");
  return methods;
};

export const NOT_LINKED_YET = "Single sign-on, not linked yet";

export const deleteUser = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/users/${name}`, {
    method: "DELETE",
    authToken,
  });
};

export const getUser = async (
  authToken: string,
  email: string,
): Promise<APIUser> => {
  return apiFetch<APIUser>(`/api/v1/users/${email}`, { authToken });
};
