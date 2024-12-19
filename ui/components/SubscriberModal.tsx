"use client";
import React, { useCallback, useEffect, useState } from "react";
import {
  ActionButton,
  Form,
  Input,
  Modal,
  Notification,
  Select,
} from "@canonical/react-components";
import { NetworkSlice } from "@/components/types";
import { createSubscriber } from "@/utils/createSubscriber";
import { editSubscriber } from "@/utils/editSubscriber";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import * as Yup from "yup";
import { useFormik } from "formik";

interface SubscriberValues {
  imsi: string;
  opc: string;
  key: string;
  sequenceNumber: string;
  selectedSlice: string;
  deviceGroup: string;
}

type Props = {
  toggleModal: () => void;
  subscriber?: any;
  slices: NetworkSlice[];
  deviceGroups: any[]
};

const SubscriberModal = ({ toggleModal, subscriber, slices, deviceGroups }: Props) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);
  const rawIMSI = subscriber?.ueId.split("-")[1];

  const oldProfile = deviceGroups.find(
    (deviceGroup) => deviceGroup["imsis"]?.includes(rawIMSI)
  );
  const oldProfileName: string = oldProfile ? oldProfile["group-name"] : "";

  const oldNetworkSlice = slices.find(
    (slice) => slice["profiles"]?.includes(oldProfileName)
  );
  const oldNetworkSliceName: string = oldNetworkSlice ? oldNetworkSlice["name"] : "";

  const SubscriberSchema = Yup.object().shape({
    imsi: Yup.string()
      .min(14)
      .max(15)
      .matches(/^[0-9]+$/, { message: "Only numbers are allowed." })
      .required("IMSI must be 14 or 15 digits"),
    opc: Yup.string()
      .length(32)
      .matches(/^[A-Za-z0-9]+$/, {
        message: "Only alphanumeric characters are allowed.",
      })
      .required("OPC must be a 32 character hexadecimal"),
    key: Yup.string()
      .length(32)
      .matches(/^[A-Za-z0-9]+$/, {
        message: "Only alphanumeric characters are allowed.",
      })
      .required("Key must be a 32 character hexadecimal"),
    sequenceNumber: Yup.string().required("Sequence number is required"),
    deviceGroup: Yup.string().required(""),
  });

  const modalTitle = () => {
    return subscriber && rawIMSI ? ("Edit Subscriber: " + rawIMSI) : "Create Subscriber"
  }

  const buttonText = () => {
    return subscriber ? "Save Changes" : "Create"
  }

  const formik = useFormik<SubscriberValues>({
    initialValues: {
      imsi: rawIMSI || "",
      opc: subscriber?.opc || "",
      key: subscriber?.key || "",
      sequenceNumber: subscriber?.sequenceNumber || "",
      selectedSlice: oldNetworkSliceName,
      deviceGroup: oldProfileName,
    },
    validationSchema: SubscriberSchema,
    onSubmit: async (values) => {
      try {
        if (subscriber) {
          await editSubscriber({
            imsi: values.imsi,
            opc: values.opc,
            key: values.key,
            sequenceNumber: values.sequenceNumber,
            oldProfileName: oldProfileName,
            newProfileName: values.deviceGroup,
          });
        } else {
          await createSubscriber({
            ueId: `imsi-${values.imsi}`,
            plmnID: "00101",
            opc: values.opc,
            key: values.key,
            sequenceNumber: values.sequenceNumber,
            deviceGroupName: values.deviceGroup,
          });
        }
        await queryClient.invalidateQueries({ queryKey: [queryKeys.subscribers] });
        await queryClient.invalidateQueries({ queryKey: [queryKeys.deviceGroups] });
        await queryClient.invalidateQueries({ queryKey: [queryKeys.networkSlices] });
        toggleModal();
      } catch (error) {
        console.error(error);
        setApiError(
          (error as Error).message || "An unexpected error occurred.",
        );
      }
    },
  });

  const handleSliceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    void formik.setFieldValue("selectedSlice", e.target.value);
  };

  const handleProfileChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    void formik.setFieldValue("deviceGroup", e.target.value);
  };

  const selectedSlice = slices.find(
    (slice) => slice["name"] === formik.values.selectedSlice,
  );

  const setProfile = useCallback(
    (deviceGroup: string) => {
      if (formik.values.deviceGroup !== deviceGroup) {
        formik.setFieldValue("deviceGroup", deviceGroup);
      }
    },
    [formik],
  );

  const deviceGroupOptions =
    selectedSlice && selectedSlice["profiles"]
      ? selectedSlice["profiles"]
      : [];

  useEffect(() => {
    if (subscriber && selectedSlice && oldNetworkSliceName == selectedSlice["name"]) {
      setProfile(oldProfileName);
    }
    else if (selectedSlice && selectedSlice["profiles"]?.length === 1) {
      setProfile(selectedSlice["profiles"][0]);
    }
    else {
      setProfile("");
    }
  }, [deviceGroupOptions]);

  return (
    <Modal
      close={toggleModal}
      title={modalTitle()}
      buttonRow={
        <>
          <ActionButton
            appearance="positive"
            className="u-no-margin--bottom"
            onClick={formik.submitForm}
            disabled={!(formik.isValid && formik.dirty)}
            loading={formik.isSubmitting}
          >
            {buttonText()}
          </ActionButton>
        </>
      }
    >
      {apiError && (
        <Notification severity="negative" title="Error">
          {apiError}
        </Notification>
      )}
      <Form stacked>
        <Input
          type="text"
          placeholder="208930100007487"
          id="imsi"
          label="IMSI"
          stacked
          required
          disabled={subscriber ? true : false}
          {...formik.getFieldProps("imsi")}
          error={formik.touched.imsi ? formik.errors.imsi : null}
        />
        <Input
          type="text"
          id="opc"
          placeholder="981d464c7c52eb6e5036234984ad0bcf"
          label="OPC"
          help="Operator code"
          stacked
          required
          {...formik.getFieldProps("opc")}
          error={formik.touched.opc ? formik.errors.opc : null}
        />
        <Input
          type="text"
          id="key"
          placeholder="5122250214c33e723a5dd523fc145fc0"
          label="Key"
          help="Permanent subscription key"
          stacked
          required
          {...formik.getFieldProps("key")}
          error={formik.touched.key ? formik.errors.key : null}
        />
        <Input
          type="text"
          id="sequence-number"
          placeholder="16f3b3f70fc2"
          label="Sequence Number"
          stacked
          required
          {...formik.getFieldProps("sequenceNumber")}
          error={
            formik.touched.sequenceNumber ? formik.errors.sequenceNumber : null
          }
        />
        <Select
          id="network-slice"
          label="Network Slice"
          stacked
          required
          value={formik.values.selectedSlice}
          onChange={handleSliceChange}
          error={
            formik.touched.selectedSlice ? formik.errors.selectedSlice : null
          }
          options={[
            {
              disabled: true,
              label: "Select an option",
              value: "",
            },
            ...slices.map((slice) => ({
              label: slice["name"],
              value: slice["name"],
            })),
          ]}
        />
        <Select
          id="device-group"
          label="Device Group"
          stacked
          required
          value={formik.values.deviceGroup}
          onChange={handleProfileChange}
          error={formik.touched.deviceGroup ? formik.errors.deviceGroup : null}
          options={[
            {
              disabled: true,
              label: "Select an option",
              value: "",
            },
            ...deviceGroupOptions.map((group) => ({
              label: group,
              value: group,
            })),
          ]}
        />
      </Form>
    </Modal>
  );
};

export default SubscriberModal;
