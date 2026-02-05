import { apiFetch } from "@/queries/utils";

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
) => {
  return apiFetch(`/api/v1/users`, {
    method: "POST",
    authToken,
    body: { email, password, role_id },
  });
};

export const updateUserPassword = async (
  authToken: string,
  email: string,
  password: string,
) => {
  return apiFetch(`/api/v1/users/${email}/password`, {
    method: "PUT",
    authToken,
    body: { password },
  });
};

export const updateMyUserPassword = async (
  authToken: string,
  password: string,
) => {
  return apiFetch(`/api/v1/users/me/password`, {
    method: "PUT",
    authToken,
    body: { password },
  });
};

export const updateUser = async (
  authToken: string,
  email: string,
  role_id: RoleID,
) => {
  return apiFetch(`/api/v1/users/${email}`, {
    method: "PUT",
    authToken,
    body: { role_id },
  });
};

export const deleteUser = async (authToken: string, name: string) => {
  return apiFetch(`/api/v1/users/${name}`, {
    method: "DELETE",
    authToken,
  });
};
