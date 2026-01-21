import { HTTPStatus } from "@/queries/utils";

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
  const response = await fetch(
    `/api/v1/users?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );
  let json: { result: ListUsersResponse; error?: string };
  try {
    json = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${json?.error || "Unknown error"}`,
    );
  }

  return json.result;
}

export const getLoggedInUser = async (authToken: string) => {
  const response = await fetch(`/api/v1/users/me`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const createUser = async (
  authToken: string,
  email: string,
  role_id: RoleID,
  password: string,
) => {
  const userData = {
    email: email,
    password: password,
    role_id: role_id,
  };

  const response = await fetch(`/api/v1/users`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const updateUserPassword = async (
  authToken: string,
  email: string,
  password: string,
) => {
  const userData = {
    password: password,
  };

  const response = await fetch(`/api/v1/users/${email}/password`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const updateMyUserPassword = async (
  authToken: string,
  password: string,
) => {
  const userData = {
    password: password,
  };

  const response = await fetch(`/api/v1/users/me/password`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const updateUser = async (
  authToken: string,
  email: string,
  role_id: RoleID,
) => {
  const userData = {
    role_id: role_id,
  };

  const response = await fetch(`/api/v1/users/${email}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const deleteUser = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/users/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};
