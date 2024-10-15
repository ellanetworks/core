"use client";

import React, { useState } from "react";
import {
  Button,
  MainTable,
  ConfirmationButton,
} from "@canonical/react-components";
import RadioModal from "@/components/RadioModal";
import { listRadios, deleteRadio } from "@/queries/radios";
import SyncOutlinedIcon from "@mui/icons-material/SyncOutlined";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";

export type Radio = {
  id: string;
  name: string;
  tac: string;
  network_slice_id: string;
};

const Radios = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);

  const { data: radios = [], isLoading: isRadiosLoading } = useQuery({
    queryKey: [queryKeys.radios],
    queryFn: listRadios,
  });


  const handleRefresh = async () => {
    await queryClient.invalidateQueries({ queryKey: [queryKeys.radios] });
  };

  const handleConfirmDelete = async (radio: string) => {
    await deleteRadio(radio);
    await handleRefresh();
  };

  const toggleCreateModal = () => setCreateModalVisible((prev) => !prev);


  const getDeleteButton = (imsi: string, radio_id: string) => {
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(radio_id),
        children: (
          <p>
            This will permanently delete the radio{" "}
            <b>{imsi}</b>
            <br />
            This action cannot be undone.
          </p>
        ),
      }}
    >
      Delete
    </ConfirmationButton>
  }

  const tableContent = radios.map((radio) => {
    return {
      key: radio.imsi,
      columns: [
        { content: radio.id },
        { content: radio.tac },
        { content: radio.network_slice_id },
        {
          content: (
            <div className="u-align--right">
              {getDeleteButton(radio.imsi, radio.id)}
            </div>
          ),
        },
      ],
    };
  });

  if (isRadiosLoading) {
    return <Loader text="Loading..." />;
  }

  return (
    <>
      <PageHeader title={`Radios (${radios.length})`}>
        <Button
          hasIcon
          appearance="base"
          onClick={handleRefresh}
          title="refresh radio list"
        >
          <SyncOutlinedIcon style={{ color: "#666" }} />
        </Button>
        <Button appearance="positive" onClick={toggleCreateModal}>
          Create
        </Button>
      </PageHeader>
      <PageContent>
        <MainTable
          defaultSort='"abcd"'
          defaultSortDirection="ascending"
          headers={[
            { content: "Id" },
            { content: "TAC" },
            { content: "Network Slice ID" },
            { content: "Actions", className: "u-align--right" },
          ]}
          rows={tableContent}
        />
      </PageContent>
      {isCreateModalVisible && <RadioModal toggleModal={toggleCreateModal} />}
    </>
  );
};
export default Radios;
