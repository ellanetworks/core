
export const apiListNetworkSlices = async () => {
    try {
        const networkSlicesResponse = await fetch(`/api/v1/network-slices`, {
            method: "GET",
            headers: {
                "Content-Type": "application/json",
            },
        });
        return networkSlicesResponse
    } catch (error) {
        console.error(error);
        throw error;
    }
};

export const apiGetNetworkSlice = async (id: string) => {
    try {
        const response = await fetch(`/api/v1/network-slices/${id}`, {
            method: "GET",
            headers: {
                "Content-Type": "application/json",
            },
        });
        return response
    } catch (error) {
        console.error(error);
        throw error;
    }
};

export const apiPostNetworkSlice = async (sliceData: any) => {
    try {
        const response = await fetch(`/api/v1/network-slices`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(sliceData),
        });
        return response
    } catch (error) {
        console.error(error);
        throw error;
    }
};

export const apiDeleteNetworkSlice = async (id: string) => {
    try {
        const response = await fetch(`/api/v1/network-slices/${id}`, {
            method: "DELETE",
            headers: {
                "Content-Type": "application/json",
            },
        });
        return response
    } catch (error) {
        console.error(error);
        throw error;
    }
};

export const deleteNetworkSlice = async (id: string) => {
    try {
        const response = await apiDeleteNetworkSlice(id);
        if (!response.ok) {
            throw new Error("Failed to delete network slice");
        }
        return true;
    } catch (error) {
        console.error(error);
        return false;
    }
};


interface CreateNetworkSliceArgs {
    name: string;
    mcc: string;
    mnc: string;
}

export const createNetworkSlice = async ({
    name,
    mcc,
    mnc,
}: CreateNetworkSliceArgs) => {
    const sliceData = {
        "name": name,
        "sst": 1,
        "sd": "102030",
        "site_name": "demo",
        "mcc": mcc,
        "mnc": mnc,
    };

    try {
        const updateNetworkSliceResponse = await apiPostNetworkSlice(sliceData);
        if (!updateNetworkSliceResponse.ok) {
            const networkSliceData = await updateNetworkSliceResponse.json();
            if (networkSliceData.error) {
                throw new Error(networkSliceData.error);
            }
            debugger;
            throw new Error(
                `Error creating network slice. Error code: ${updateNetworkSliceResponse.status}`,
            );
        }

        return updateNetworkSliceResponse.json();
    } catch (error: unknown) {
        console.error(error);
        const details =
            error instanceof Error
                ? error.message
                : "Failed to configure the network.";
        throw new Error(details);
    }
};

export const listNetworkSlices = async () => {
    try {
        const response = await apiListNetworkSlices();
        if (!response.ok) {
            throw new Error(
                `Failed to fetch network slices. Status: ${response.status}`,
            );
        }

        var jsonResponse = await response.json();
        // Response format:
        // {"result":[1,2,3]}
        console.log("list network slices:", jsonResponse)
        if (!jsonResponse["result"]) {
            throw new Error("Failed to fetch network slices.");
        }

        var allNetworkSlices = await Promise.all(
            jsonResponse["result"].map(async (id: string) => {
                return await getNetworkSlice(id);
            }),
        );

        return allNetworkSlices.filter((item) => item !== undefined);
    } catch (error) {
        console.error(error);
        throw error;
    }
};

const getNetworkSlice = async (id: string) => {
    try {
        const response = await apiGetNetworkSlice(id);
        if (!response.ok)
            throw new Error(
                `Failed to fetch network slice. Status: ${response.status}`,
            );
        const responseJson = await response.json();
        // Response format:
        console.log("get network slice:", responseJson)
        if (!responseJson["result"]) {
            throw new Error("Failed to fetch network slices.");
        }
        return responseJson["result"];
    } catch (error) {
        console.error(error);
    }
};
