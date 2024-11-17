"use client";

import React, { useState } from "react";
import {
  Button,
  MainTable,
  ConfirmationButton,
} from "@canonical/react-components";
import UPFModal from "@/components/UPFModal";
import { listUPFs, deleteUPF } from "@/queries/upfs";
import SyncOutlinedIcon from "@mui/icons-material/SyncOutlined";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";

export type UPF = {
  id: string;
  name: string;
  tac: string;
  network_slice_id: string;
};

const UPFs = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);

  const { data: upfs = [], isLoading: isUPFsLoading } = useQuery({
    queryKey: [queryKeys.upfs],
    queryFn: listUPFs,
  });


  const handleRefresh = async () => {
    await queryClient.invalidateQueries({ queryKey: [queryKeys.upfs] });
  };

  const handleConfirmDelete = async (upf: string) => {
    await deleteUPF(upf);
    await handleRefresh();
  };

  const toggleCreateModal = () => setCreateModalVisible((prev) => !prev);


  const getDeleteButton = (imsi: string, upf_id: string) => {
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(upf_id),
        children: (
          <p>
            This will permanently delete the upf{" "}
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

  const tableContent = upfs.map((upf) => {
    return {
      key: upf.imsi,
      columns: [
        { content: upf.id },
        { content: upf.tac },
        { content: upf.network_slice_id },
        {
          content: (
            <div className="u-align--right">
              {getDeleteButton(upf.imsi, upf.id)}
            </div>
          ),
        },
      ],
    };
  });

  if (isUPFsLoading) {
    return <Loader text="Loading..." />;
  }

  return (
    <>
      <PageHeader title={`UPFs (${upfs.length})`}>
        <Button
          hasIcon
          appearance="base"
          onClick={handleRefresh}
          title="refresh upf list"
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
      {isCreateModalVisible && <UPFModal toggleModal={toggleCreateModal} />}
    </>
  );
};
export default UPFs;
