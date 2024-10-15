"use client";
import React, { useState } from "react";
import {
  ActionButton,
  Form,
  Input,
  Modal,
  Notification,
  Select,
} from "@canonical/react-components";
import { createRadio } from "@/queries/radios";
import { listNetworkSlices } from "@/queries/networkSlices";
import { useQueryClient, useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import * as Yup from "yup";
import { useFormik } from "formik";

interface RadioValues {
  name: string;
  tac: string;
  networkSliceId: number;
}

type Props = {
  toggleModal: () => void;
  radio?: any;
};

const RadioModal = ({ toggleModal, radio }: Props) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);


  const { data: networkSlices = [], isLoading: isNetworkSlicesLoading } = useQuery({
    queryKey: [queryKeys.networkSlices],
    queryFn: listNetworkSlices,
  });

  const RadioSchema = Yup.object().shape({
    name: Yup.string()
      .min(1)
      .max(15)
      .required("Name is required"),
    tac: Yup.string()
      .min(1)
      .max(5)
      .required("TAC is required"),

  });

  const buttonText = () => {
    return radio ? "Save Changes" : "Create"
  }

  const handleNetworkSliceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    formik.setFieldValue("networkSliceId", parseInt(e.target.value, 10));
  };

  const formik = useFormik<RadioValues>({
    initialValues: {
      name: radio?.["name"] || "",
      tac: radio?.["tac"] || "",
      networkSliceId: radio?.["networkSliceId"] || "",
    },
    validationSchema: RadioSchema,
    onSubmit: async (values) => {
      try {
        await createRadio({
          name: values.name,
          tac: values.tac,
          network_slice_id: values.networkSliceId,
        });
        await queryClient.invalidateQueries({ queryKey: [queryKeys.radios] });
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
      close={toggleModal}
      title={"Create Radio"}
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
          placeholder="gnb-1"
          id="name"
          label="Name"
          stacked
          required
          disabled={radio ? true : false}
          {...formik.getFieldProps("name")}
          error={formik.touched.name ? formik.errors.name : null}
        />
        <Input
          type="text"
          id="tac"
          placeholder="001"
          label="Tracking Area Code"
          help="Unique identifier for the tracking area"
          stacked
          required
          {...formik.getFieldProps("tac")}
          error={formik.touched.tac ? formik.errors.tac : null}
        />
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

export default RadioModal;
