import { HTTPStatus } from "@/queries/utils";

export const login = async (email: string, password: string) => {
    const loginData = {
        "email": email,
        "password": password,
    }
    const response = await fetch(`/api/v1/auth/login`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(loginData),
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

export const lookupToken = async (authToken: string) => {
    const response = await fetch(`/api/v1/auth/lookup-token`, {
        method: "POST",
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
