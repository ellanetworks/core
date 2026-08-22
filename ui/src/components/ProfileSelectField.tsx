// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useQuery } from "@tanstack/react-query";
import { useController } from "react-hook-form";
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
  control,
  name,
  profiles,
}: ProfileSelectFieldProps<T>) => {
  const { field } = useController({ control, name });
  const current = field.value as string | undefined;
  const options =
    current && !profiles.includes(current) ? [current, ...profiles] : profiles;

  return (
    <SelectControl<T, string>
      name={name}
      label="Profile"
      options={options.map((profile) => ({ value: profile, label: profile }))}
    />
  );
};

export default ProfileSelectField;
