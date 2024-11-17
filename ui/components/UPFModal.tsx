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
import { createUPF } from "@/queries/upfs";
import { listNetworkSlices } from "@/queries/networkSlices";
import { useQueryClient, useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import * as Yup from "yup";
import { useFormik } from "formik";

interface UPFValues {
  name: string;
  networkSliceId: number;
}

type Props = {
  toggleModal: () => void;
  upf?: any;
};

const UPFModal = ({ toggleModal, upf }: Props) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);


  const { data: networkSlices = [], isLoading: isNetworkSlicesLoading } = useQuery({
    queryKey: [queryKeys.networkSlices],
    queryFn: listNetworkSlices,
  });

  const UPFSchema = Yup.object().shape({
    name: Yup.string()
      .min(1)
      .max(15)
      .required("Name is required"),
  });

  const buttonText = () => {
    return upf ? "Save Changes" : "Create"
  }

  const handleNetworkSliceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    formik.setFieldValue("networkSliceId", parseInt(e.target.value, 10));
  };

  const formik = useFormik<UPFValues>({
    initialValues: {
      name: upf?.["name"] || "",
      networkSliceId: upf?.["networkSliceId"] || "",
    },
    validationSchema: UPFSchema,
    onSubmit: async (values) => {
      try {
        await createUPF({
          name: values.name,
          network_slice_id: values.networkSliceId,
        });
        await queryClient.invalidateQueries({ queryKey: [queryKeys.upfs] });
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
      title={"Create UPF"}
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
          placeholder="upf-1"
          id="name"
          label="Name"
          stacked
          required
          disabled={upf ? true : false}
          {...formik.getFieldProps("name")}
          error={formik.touched.name ? formik.errors.name : null}
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

export default UPFModal;
