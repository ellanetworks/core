import React, { useState } from "react";
import {
  Input,
  Notification,
  Modal,
  Form,
  ActionButton,
} from "@canonical/react-components";
import { NetworkSlice } from "@/components/types";
import { createNetworkSlice } from "@/queries/networkSlices";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import { useFormik } from "formik";
import * as Yup from "yup";

interface NetworkSliceValues {
  mcc: string;
  mnc: string;
  name: string;
}

interface NetworkSliceModalProps {
  networkSlice?: NetworkSlice;
  toggleModal: () => void;
}

const NetworkSliceModal = ({ networkSlice, toggleModal }: NetworkSliceModalProps) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);

  const NetworkSliceSchema = Yup.object().shape({
    name: Yup.string()
      .min(1)
      .max(20, "Name should not exceed 20 characters")
      .matches(/^[a-zA-Z0-9-_]+$/, {
        message: "Only alphanumeric characters, dashes and underscores.",
      })
      .required("Name is required."),
    mcc: Yup.string()
      .matches(/^\d{3}$/, "MCC must be 3 digits.")
      .length(3)
      .required("MCC is required."),
    mnc: Yup.string()
      .matches(/^\d{2,3}$/, "MNC must be 2 or 3 digits.")
      .min(2)
      .max(3)
      .required("MNC is required."),
  });

  const buttonText = () => {
    return networkSlice ? "Save Changes" : "Create"
  }

  const formik = useFormik<NetworkSliceValues>({
    initialValues: {
      mcc: networkSlice?.mcc || "",
      mnc: networkSlice?.mnc || "",
      name: networkSlice?.["name"] || "",
    },
    validationSchema: NetworkSliceSchema,
    onSubmit: async (values) => {
      try {
        await createNetworkSlice({
          name: values.name,
          mcc: values.mcc.toString(),
          mnc: values.mnc.toString(),
        });
        await queryClient.invalidateQueries({
          queryKey: [queryKeys.networkSlices],
        });
        toggleModal();
      } catch (error) {
        console.error(error);
        setApiError(
          (error as Error).message || "An unexpected error occurred.",
        );
      }
    },
  });

  return (
    <Modal
      title={"Create Network Slice"}
      close={toggleModal}
      buttonRow={
        <ActionButton
          appearance="positive"
          className="u-no-margin--bottom mt-8"
          onClick={formik.submitForm}
          disabled={!(formik.isValid && formik.dirty)}
          loading={formik.isSubmitting}
        >
          {buttonText()}
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
          disabled={networkSlice ? true : false}
          {...formik.getFieldProps("name")}
          error={formik.touched.name ? formik.errors.name : null}
        />
        <Input
          type="text"
          id="mcc"
          label="MCC"
          help="Mobile Country Code"
          placeholder="001"
          stacked
          required
          {...formik.getFieldProps("mcc")}
          error={formik.touched.mcc ? formik.errors.mcc : null}
        />
        <Input
          type="text"
          id="mnc"
          label="MNC"
          help="Mobile Network Code (2 or 3 digits)"
          placeholder="01"
          stacked
          required
          {...formik.getFieldProps("mnc")}
          error={formik.touched.mnc ? formik.errors.mnc : null}
        />
      </Form>
    </Modal>
  );
};

export default NetworkSliceModal;
