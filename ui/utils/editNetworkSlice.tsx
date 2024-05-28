interface GnbItem {
    name: string;
    tac: number;
  }
  
  interface EditNetworkSliceArgs {
    name: string;
    mcc: string;
    mnc: string;
    upfName: string;
    upfPort: string;
    gnbList: GnbItem[];
  }
  
  export const editNetworkSlice = async ({
    name,
    mcc,
    mnc,
    upfName,
    upfPort,
    gnbList,
  }: EditNetworkSliceArgs) => {
  
    try {
      const checkResponse = await fetch(`/api/network-slice/${name}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      });
  
      if (!checkResponse.ok) {
        const result = await checkResponse.json();
        if (result.error) {
          throw new Error(result.error);
        }
        throw new Error("Error editing Network Slice " + name);
      }

      var slice = await checkResponse.json();
      slice["site-info"]["plmn"].mcc = mcc
      slice["site-info"]["plmn"].mnc = mnc
      slice["site-info"]["gNodeBs"] = gnbList
      slice["site-info"]["upf"]["upf-name"] = upfName
      slice["site-info"]["upf"]["upf-port"] = upfPort
  
      const networksliceResponse = await fetch(`/api/network-slice/${name}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(slice),
      });
  
      if (!networksliceResponse.ok) {
        const result = await networksliceResponse.json();
        if (result.error) {
          throw new Error(result.error);
        }
        debugger;
        throw new Error(
          `Error editing network. Error code: ${networksliceResponse.status}`,
        );
      }
  
      return networksliceResponse.json();
    } catch (error: unknown) {
      console.error(error);
      const details =
        error instanceof Error
          ? error.message
          : "Failed to configure the network.";
      throw new Error(details);
    }
  };
  