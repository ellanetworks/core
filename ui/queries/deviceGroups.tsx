import internal from "stream";

export const apiListDeviceGroups = async () => {
    try {
        const response = await fetch(`/api/v1/device-groups`, {
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

export const apiGetDeviceGroup = async (id: string) => {
    try {
        const response = await fetch(`/api/v1/device-groups/${id}`, {
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

export const apiPostDeviceGroup = async (deviceGroupData: any) => {
    try {
        const response = await fetch(`/api/v1/device-groups`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(deviceGroupData),
        });
        return response
    } catch (error) {
        console.error(error);
        throw error;
    }
};

export const apiDeleteDeviceGroup = async (id: string) => {
    try {
        const response = await fetch(`/api/v1/device-groups/${id}`, {
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


export const deleteDeviceGroup = async (id: string) => {
    try {
        const deleteResponse = await apiDeleteDeviceGroup(id);
        if (!deleteResponse.ok) {
            throw new Error(
                `Error deleting device group. Error code: ${deleteResponse.status}`,
            );
        }

        return true;
    } catch (error) {
        console.error(error);
        throw new Error("Failed to delete device group.");
    }
};

export const listDeviceGroups = async () => {
    try {
        const response = await apiListDeviceGroups();
        if (!response.ok) {
            throw new Error(
                `Failed to fetch device groups. Status: ${response.status}`,
            );
        }

        var jsonResponse = await response.json();
        // Response format:
        // {"result":[1,2,3]}
        console.log("list-device groups:", jsonResponse)
        if (!jsonResponse["result"]) {
            throw new Error("Failed to fetch device groups.");
        }


        var allDeviceGroups = await Promise.all(
            jsonResponse["result"].map(async (id: string) => {
                return await getDeviceGroup(id);
            }),
        );

        return allDeviceGroups.filter((item) => item !== undefined);
    } catch (error) {
        console.error(error);
        throw error;
    }
};

const getDeviceGroup = async (id: string) => {
    try {
        const response = await apiGetDeviceGroup(id);
        if (!response.ok)
            throw new Error(
                `Failed to fetch device group. Status: ${response.status}`,
            );
        const responseJson = await response.json();
        // Response format:
        // {"result":{"id":1,"name":"default","site_info":"demo","ip_domain_name":"pool1","dnn":"internet","ue_ip_pool":"172.250.1.0/16","dns_primary":"8.8.8.8","mtu":1460,"dnn_mbr_uplink":2000000,"dnn_mbr_downlink":22000000,"traffic_class_name":"platinum","traffic_class_arp":6,"traffic_class_pdb":300,"traffic_class_pelr":6,"traffic_class_qci":8}}
        console.log("get-device group:", responseJson)
        if (!responseJson["result"]) {
            throw new Error("Failed to fetch device groups.");
        }

        return responseJson["result"];
    } catch (error) {
        console.error(error);
    }
};

interface CreateDeviceGroupArgs {
    name: string;
    ueIpPool: string;
    dns: string;
    mtu: number;
    MBRUpstreamBps: number;
    MBRDownstreamBps: number;
    NetworkSliceId: number;
}

export const createDeviceGroup = async ({
    name,
    ueIpPool,
    dns,
    mtu,
    MBRUpstreamBps,
    MBRDownstreamBps,
    NetworkSliceId,
}: CreateDeviceGroupArgs) => {
    const deviceGroupData = {
        "name": name,
        "site_info": "demo",
        "ip_domain_name": "pool1",
        "dnn": "internet",
        "ue_ip_pool": ueIpPool,
        "dns_primary": dns,
        "mtu": mtu,
        "dnn_mbr_uplink": MBRUpstreamBps,
        "dnn_mbr_downlink": MBRDownstreamBps,
        "traffic_class_name": "platinum",
        "traffic_class_arp": 6,
        "traffic_class_pdb": 300,
        "traffic_class_pelr": 6,
        "traffic_class_qci": 8,
        "network_slice_id": NetworkSliceId,
    };

    try {

        const updateDeviceGroupResponse = await apiPostDeviceGroup(deviceGroupData);
        if (!updateDeviceGroupResponse.ok) {
            throw new Error(
                `Error creating device group. Error code: ${updateDeviceGroupResponse.status}`,
            );
        }

        return true;
    } catch (error: unknown) {
        console.error(error);
        const details =
            error instanceof Error
                ? error.message
                : "Failed to configure the network.";
        throw new Error(details);
    }
};
