import React, { useEffect, useState } from "react";
import {
  Input,
  Notification,
  Modal,
  Form,
  Select,
  ActionButton,
} from "@canonical/react-components";
import { NetworkSlice } from "@/components/types";
import { createNetworkSlice } from "@/utils/createNetworkSlice";
import { editNetworkSlice } from "@/utils/editNetworkSlice";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import { getGnbList, GnbItem } from "@/utils/getRadioList";
import { useFormik } from "formik";
import * as Yup from "yup";

interface NetworkSliceValues {
  mcc: string;
  mnc: string;
  name: string;
  radioList: GnbItem[];
}

interface NetworkSliceModalProps {
  networkSlice?: NetworkSlice;
  toggleModal: () => void;
}

const NetworkSliceModal = ({ networkSlice, toggleModal }: NetworkSliceModalProps) => {
  const queryClient = useQueryClient();
  const [apiError, setApiError] = useState<string | null>(null);
  const [radioApiError, setGnbApiError] = useState<string | null>(null);

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
    radioList: Yup.array()
      .min(1)
      .required("Selecting at least 1 gNodeB is required."),
  });

  const modalTitle = () => {
    return networkSlice?.["name"] ? ("Edit Network Slice: " + networkSlice["name"]) : "Create Network Slice"
  }

  const buttonText = () => {
    return networkSlice ? "Save Changes" : "Create"
  }

  const formik = useFormik<NetworkSliceValues>({
    initialValues: {
      mcc: networkSlice?.mcc || "",
      mnc: networkSlice?.mnc || "",
      name: networkSlice?.["name"] || "",
      radioList: networkSlice?.gNodeBs || [],
    },
    validationSchema: NetworkSliceSchema,
    onSubmit: async (values) => {
      try {
        if (networkSlice) {
          await editNetworkSlice({
            name: values.name,
            mcc: values.mcc.toString(),
            mnc: values.mnc.toString(),
            upfName: "0.0.0.0",
            upfPort: 8806,
            radioList: values.radioList,
          });
        } else {
          await createNetworkSlice({
            name: values.name,
            mcc: values.mcc.toString(),
            mnc: values.mnc.toString(),
            upfName: "0.0.0.0",
            upfPort: 8806,
            radioList: values.radioList,
          });
        }
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

  const { data: radioList = [], isLoading: isGnbLoading } = useQuery({
    queryKey: [queryKeys.radioList],
    queryFn: getGnbList,
  });

  useEffect(() => {
    const checkGnbList = async () => {
      if (!isGnbLoading && radioList.length === 0) {
        setGnbApiError("Failed to retrieve the list of GNBs from the server.");
      }
    };
    checkGnbList();
  }, [isGnbLoading, radioList]);

  const handleGnbChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const selectedOptions = Array.from(e.target.selectedOptions);
    const items = radioList.filter((item) =>
      selectedOptions.some(
        (option) => option.value === `${item.name}:${item.tac}`,
      ),
    );
    void formik.setFieldValue("radioList", items);
  };

  const getGnbListValueAsString = () => {
    return (formik.values.radioList.map((item) => {
      return `${item.name}:${item.tac}`
    }));
  };

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
          {buttonText()}
        </ActionButton>
      }
    >
      {apiError && (
        <Notification severity="negative" title="Error">
          {apiError}
        </Notification>
      )}
      {radioApiError && (
        <Notification severity="negative" title="Error">
          {radioApiError}
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
        <Select
          id="radio"
          stacked
          required
          value={getGnbListValueAsString()}
          options={[
            {
              value: "",
              disabled: true,
              label: "Select...",
            },
            ...radioList.map((radio) => ({
              label: `${radio.name} (tac:${radio.tac})`,
              value: `${radio.name}:${radio.tac}`,
            })),
          ]}
          label="gNodeBs"
          onChange={handleGnbChange}
          multiple
        />
      </Form>
    </Modal>
  );
};

export default NetworkSliceModal;
