import React, { createContext, useContext, ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { getStatus, type APIFleetStatus } from "@/queries/status";

interface FleetContextType {
  isFleetManaged: boolean;
  lastSyncAt: string | undefined;
}

const FleetContext = createContext<FleetContextType>({
  isFleetManaged: false,
  lastSyncAt: undefined,
});

export const FleetProvider = ({ children }: { children: ReactNode }) => {
  const { data } = useQuery({
    queryKey: ["status", "fleet"],
    queryFn: getStatus,
    refetchInterval: 5000,
    staleTime: 4000,
  });

  const fleet: APIFleetStatus = data?.fleet ?? { managed: false };

  return (
    <FleetContext.Provider
      value={{
        isFleetManaged: fleet.managed,
        lastSyncAt: fleet.lastSyncAt,
      }}
    >
      {children}
    </FleetContext.Provider>
  );
};

export const useFleet = () => useContext(FleetContext);
