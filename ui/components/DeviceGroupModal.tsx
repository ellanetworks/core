import React, { useState } from "react";
import {
  Input,
  Notification,
  Modal,
  Select,
  Form,
  ActionButton,
} from "@canonical/react-components";
import { createDeviceGroup } from "@/queries/deviceGroups";
import { listNetworkSlices } from "@/queries/networkSlices";
import * as Yup from "yup";
import { useFormik } from "formik";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";

const regexIp =
  /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)$/;

const regexpCIDR =
  /^((25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\/([1-9]|[1-2][0-9]|3[0-2])$/;

interface DeviceGroupValues {
  name: string;
  ueIpPool: string;
  dns: string;
  mtu: number;
  MBRDownstreamMbps: number | null;
  MBRUpstreamMbps: number | null;
  networkSliceId: number;
}

interface DeviceGroupModalProps {
  toggleModal: () => void;
  deviceGroup?: any;
}

const ModalButtonText = (deviceGroupName: string | undefined) => {
  return deviceGroupName ? "Save Changes" : "Create"
}

const DeviceGroupModal = ({
  toggleModal,
  deviceGroup,
}: DeviceGroupModalProps) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);
  const modalButtonText = ModalButtonText(deviceGroup?.["group-name"])

  const modalTitle = () => {
    return deviceGroup && deviceGroup.name ? ("Edit Device Group: " + deviceGroup.name) : "Create Device Group"
  }

  const { data: networkSlices = [], isLoading: isNetworkSlicesLoading } = useQuery({
    queryKey: [queryKeys.networkSlices],
    queryFn: listNetworkSlices,
  });

  const DeviceGroupSchema = Yup.object().shape({
    name: Yup.string()
      .min(1)
      .max(20, "Name should not exceed 20 characters.")
      .matches(/^[a-zA-Z0-9-_]+$/, {
        message: "Only alphanumeric characters, dashes and underscores.",
      })
      .required("Name is required."),
    ueIpPool: Yup.string()
      .required("IP is required")
      .matches(regexpCIDR, "Invalid IP Address Pool."),
    dns: Yup.string()
      .required("IP is required")
      .matches(regexIp, "Invalid IP Address."),
    mtu: Yup.number().min(1200).max(65535).required("Invalid MTU."),
    MBRDownstreamMbps: Yup.number()
      .min(0)
      .max(1000000)
      .required("Value should be between 0 and 1,000,000."),
    MBRUpstreamMbps: Yup.number()
      .min(0)
      .max(1000000)
      .required("Value should be between 0 and 1,000,000."),
    networkSliceId: Yup.number()
      .required("Please select a network slice."),
  });

  const formik = useFormik<DeviceGroupValues>({
    initialValues: {
      name: deviceGroup?.["name"] || "",
      ueIpPool: deviceGroup?.["ue_ip_pool"] || "",
      dns: deviceGroup?.["dns_primary"] || "8.8.8.8",
      mtu: deviceGroup?.["mtu"] || 1460,
      MBRDownstreamMbps: deviceGroup?.["DnnMbrDownlink"] / 1_000_000 || null,
      MBRUpstreamMbps: deviceGroup?.["DnnMbrUplink"] / 1_000_000 || null,
      networkSliceId: deviceGroup?.["networkSliceId"] || "",
    },

    validationSchema: DeviceGroupSchema,
    onSubmit: async (values) => {
      const MBRUpstreamBps = Number(values.MBRUpstreamMbps) * 1000000;
      const MBRDownstreamBps = Number(values.MBRDownstreamMbps) * 1000000;
      try {
        await createDeviceGroup({
          name: values.name,
          ueIpPool: values.ueIpPool,
          dns: values.dns,
          mtu: values.mtu,
          MBRUpstreamBps: MBRUpstreamBps,
          MBRDownstreamBps: MBRDownstreamBps,
          NetworkSliceId: values.networkSliceId,
        });
        await queryClient.invalidateQueries({ queryKey: [queryKeys.deviceGroups] });
        toggleModal();
      } catch (error) {
        console.error(error);
        setApiError(
          (error as Error).message || "An unexpected error occurred.",
        );
      }
    },
  });

  const handleNetworkSliceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    formik.setFieldValue("networkSliceId", parseInt(e.target.value, 10));
  };

  if (!modalTitle) {
    return (
      <Modal
        title={"Error while trying to edit device group"}
        close={toggleModal}
      >
        An unexpected error occurred.
      </Modal>
    )
  }
  return (
    <Modal
      title={modalTitle()}
      close={toggleModal}
      buttonRow={
        <ActionButton
          appearance="positive"
          className="u-no-margin--bottom mt-8"
          onClick={formik.submitForm}
          disabled={!(formik.isValid && formik.dirty)}
          loading={formik.isSubmitting}
        >
          {modalButtonText}
        </ActionButton>
      }
    >
      {apiError && (
        <Notification severity="negative" title="Error">
          {apiError}
        </Notification>
      )}
      <Form>
        <Input
          type="text"
          id="name"
          label="Name"
          placeholder="default"
          stacked
          required
          {...formik.getFieldProps("name")}
          error={formik.touched.name ? formik.errors.name : null}
          disabled={deviceGroup}
        />
        <Input
          type="text"
          id="ue-ip-pool"
          label="Subscriber IP Pool"
          placeholder="172.250.1.0/16"
          stacked
          required
          {...formik.getFieldProps("ueIpPool")}
          error={formik.touched.ueIpPool ? formik.errors.ueIpPool : null}
        />
        <Input
          type="text"
          id="dns"
          label="DNS"
          defaultValue={"8.8.8.8"}
          stacked
          required
          {...formik.getFieldProps("dns")}
          error={formik.touched.dns ? formik.errors.dns : null}
        />
        <Input
          type="number"
          id="mtu"
          label="MTU"
          defaultValue={1460}
          stacked
          required
          {...formik.getFieldProps("mtu")}
          error={formik.touched.mtu ? formik.errors.mtu : null}
        />
        <fieldset>
          <legend>Maximum Bitrate (Mbps)</legend>
          <Input
            placeholder="20"
            id="mbr-downstream"
            type="number"
            stacked
            required
            label="Downstream"
            {...formik.getFieldProps("MBRDownstreamMbps")}
            error={
              formik.touched.MBRDownstreamMbps
                ? formik.errors.MBRDownstreamMbps
                : null
            }
          />
          <Input
            placeholder="5"
            id="mbr-upstream"
            type="number"
            stacked
            required
            label="Upstream"
            {...formik.getFieldProps("MBRUpstreamMbps")}
            error={
              formik.touched.MBRUpstreamMbps
                ? formik.errors.MBRUpstreamMbps
                : null
            }
          />
        </fieldset>
        <Select
          id="network_slices"
          stacked
          required
          value={formik.values.networkSliceId}
          options={[
            {
              value: "",
              disabled: true,
              label: "Select...",
            },
            ...networkSlices.map((networkSlice) => ({
              label: `${networkSlice.name}`,
              value: networkSlice.id,
            })),
          ]}
          label="Network Slice"
          error={formik.touched.networkSliceId ? formik.errors.networkSliceId : null}
          onChange={handleNetworkSliceChange}
        />
      </Form>
    </Modal>
  );
};
export default DeviceGroupModal;
