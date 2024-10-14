"use client";
import React, { useState } from "react";
import {
  ActionButton,
  Form,
  Input,
  Modal,
  Notification,
} from "@canonical/react-components";
import { createSubscriber } from "@/queries/subscribers";
import { editSubscriber } from "@/utils/editSubscriber";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import * as Yup from "yup";
import { useFormik } from "formik";

interface SubscriberValues {
  imsi: string;
  plmn_id: string;
  opc: string;
  key: string;
  sequence_number: string;
}

type Props = {
  toggleModal: () => void;
  subscriber?: any;
};

const SubscriberModal = ({ toggleModal, subscriber }: Props) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);

  const SubscriberSchema = Yup.object().shape({
    imsi: Yup.string()
      .min(14)
      .max(15)
      .matches(/^[0-9]+$/, { message: "Only numbers are allowed." })
      .required("IMSI must be 14 or 15 digits"),
    plmn_id: Yup.string()
      .length(5)
      .matches(/^[0-9]+$/, { message: "Only numbers are allowed." })
      .required("PLMN ID must be 5 digits"),
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
    sequence_number: Yup.string().required("Sequence number is required"),
  });

  const modalTitle = () => {
    return subscriber && subscriber.imsi ? ("Edit Subscriber: " + subscriber.imsi) : "Create Subscriber"
  }

  const buttonText = () => {
    return subscriber ? "Save Changes" : "Create"
  }

  const formik = useFormik<SubscriberValues>({
    initialValues: {
      imsi: subscriber?.["imsi"] || "",
      plmn_id: subscriber?.["plmn_id"] || "",
      opc: subscriber?.["opc"] || "",
      key: subscriber?.["key"] || "",
      sequence_number: subscriber?.["sequence_number"] || "",
    },
    validationSchema: SubscriberSchema,
    onSubmit: async (values) => {
      try {
        if (subscriber) {
          await editSubscriber({
            imsi: values.imsi,
            opc: values.opc,
            key: values.key,
            sequence_number: values.sequence_number,
          });
        } else {
          await createSubscriber({
            imsi: values.imsi,
            plmn_id: values.plmn_id,
            opc: values.opc,
            key: values.key,
            sequence_number: values.sequence_number,
          });
        }
        await queryClient.invalidateQueries({ queryKey: [queryKeys.subscribers] });
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
          placeholder="001010100007487"
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
          id="plmn_id"
          placeholder="00101"
          label="PLMN ID"
          help="Public Land Mobile Network ID"
          stacked
          required
          {...formik.getFieldProps("plmn_id")}
          error={formik.touched.plmn_id ? formik.errors.plmn_id : null}
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
          {...formik.getFieldProps("sequence_number")}
          error={
            formik.touched.sequence_number ? formik.errors.sequence_number : null
          }
        />
      </Form>
    </Modal>
  );
};

export default SubscriberModal;
