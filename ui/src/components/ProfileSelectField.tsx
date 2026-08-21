// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useQuery } from "@tanstack/react-query";
import type { Control, FieldValues, Path } from "react-hook-form";
import { useAuth } from "@/contexts/AuthContext";
import { listProfiles, type APIProfile } from "@/queries/profiles";
import SelectControl from "@/components/form/SelectControl";

export const useProfileNames = (enabled: boolean) => {
  const { accessToken, authReady } = useAuth();

  return useQuery({
    queryKey: ["profile-names"],
    queryFn: async () => {
      const page = await listProfiles(accessToken!, 1, 100);
      return (page.items ?? []).map((profile: APIProfile) => profile.name);
    },
    enabled: enabled && authReady && !!accessToken,
  });
};

interface ProfileSelectFieldProps<T extends FieldValues> {
  control: Control<T>;
  name: Path<T>;
  profiles: string[];
}

const ProfileSelectField = <T extends FieldValues>({
  name,
  profiles,
}: ProfileSelectFieldProps<T>) => (
  <SelectControl<T, string>
    name={name}
    label="Profile"
    options={profiles.map((profile) => ({ value: profile, label: profile }))}
  />
);

export default ProfileSelectField;
