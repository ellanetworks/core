"use client";

import React, { useState } from "react";
import {
  Button,
  MainTable,
  ConfirmationButton,
} from "@canonical/react-components";
import NetworkSliceModal from "@/components/NetworkSliceModal";
import { listNetworkSlices, deleteNetworkSlice } from "@/queries/networkSlices";
import SyncOutlinedIcon from "@mui/icons-material/SyncOutlined";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";

const NetworkSlices = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);

  const { data: network_slices = [], isLoading: isNetworkSlicesLoading } = useQuery({
    queryKey: [queryKeys.networkSlices],
    queryFn: listNetworkSlices,
  });


  const handleRefresh = async () => {
    await queryClient.invalidateQueries({ queryKey: [queryKeys.networkSlices] });
  };

  const handleConfirmDelete = async (id: string) => {
    await deleteNetworkSlice(id);
    await handleRefresh();
  };

  const toggleCreateModal = () => setCreateModalVisible((prev) => !prev);

  const getDeleteButton = (name: string, network_slice_id: string) => {
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(network_slice_id),
        children: (
          <p>
            This will permanently delete the network slice{" "}
            <b>{name}</b>
            <br />
            This action cannot be undone.
          </p>
        ),
      }}
    >
      Delete
    </ConfirmationButton>
  }

  const tableContent = network_slices.map((network_slice) => {
    return {
      key: network_slice.name,
      columns: [
        { content: network_slice.id },
        { content: network_slice?.["name"] },
        { content: network_slice?.["mcc"] },
        { content: network_slice?.["mnc"] },
        {
          content: (
            <div className="u-align--right">
              {getDeleteButton(network_slice.name, network_slice.id)}
            </div>
          ),
        },
      ],
    };
  });

  if (isNetworkSlicesLoading) {
    return <Loader text="Loading..." />;
  }

  console.log("Network Slices", network_slices);

  return (
    <>
      <PageHeader title={`Network Slices (${network_slices.length})`}>
        <Button
          hasIcon
          appearance="base"
          onClick={handleRefresh}
          title="refresh network slices"
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
            { content: "Name" },
            { content: "MCC" },
            { content: "MNC" },
            { content: "Actions", className: "u-align--right" },
          ]}
          rows={tableContent}
        />
      </PageContent>
      {isCreateModalVisible && <NetworkSliceModal toggleModal={toggleCreateModal} />}
    </>
  );
};
export default NetworkSlices;
