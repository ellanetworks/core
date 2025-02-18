import { HTTPStatus } from "@/queries/utils";


export const getLoggedInUser = async (authToken: string) => {
  const response = await fetch(`/api/v1/users/me`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
};


export const listUsers = async (authToken: string) => {
  const response = await fetch(`/api/v1/users`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
};

export const createUser = async (authToken: string, email: string, role: number, password: string) => {
  const userData = {
    "email": email,
    "password": password,
    "role": role,
  }

  const response = await fetch(`/api/v1/users`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
};

export const updateUserPassword = async (authToken: string, email: string, password: string) => {
  const userData = {
    "email": email,
    "password": password,
  }

  const response = await fetch(`/api/v1/users/${email}/password`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
}

export const updateUser = async (authToken: string, email: string, role: number) => {
  const userData = {
    "email": email,
    "role": role,
  }

  const response = await fetch(`/api/v1/users/${email}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
    body: JSON.stringify(userData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
}

export const deleteUser = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/users/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + authToken
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`);
  }

  return respData.result;
}