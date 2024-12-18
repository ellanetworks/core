"use client";
import React, { useState } from "react";
import {
  Button,
  Card,
  ConfirmationButton,
} from "@canonical/react-components";
import { deleteNetworkSlice } from "@/utils/deleteNetworkSlice";
import { getNetworkSlices } from "@/utils/getNetworkSlices";
import NetworkSliceModal from "@/components/NetworkSliceModal";
import NetworkSliceEmptyState from "@/components/NetworkSliceEmptyState";
import { NetworkSliceTable } from "@/components/NetworkSliceTable";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";
import { NetworkSlice } from "@/components/types";

const NetworkConfiguration = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);
  const [isEditModalVisible, setEditModalVisible] = useState(false);
  const [networkSlice, setNetworkSlice] = useState<NetworkSlice | undefined>(undefined);

  const { data: networkSlices = [], isLoading: loading } = useQuery({
    queryKey: [queryKeys.networkSlices],
    queryFn: getNetworkSlices,
  });

  const toggleCreateNetworkSliceModal = () =>
    setCreateModalVisible((prev) => !prev);

  const toggleEditNetworkSliceModal = () =>
    setEditModalVisible((prev) => !prev);

  const handleConfirmDelete = async (sliceName: string) => {
    await deleteNetworkSlice(sliceName);
    void queryClient.invalidateQueries({ queryKey: [queryKeys.networkSlices] });
  };

  const handleEditButton = (networkSlice: NetworkSlice) => {
    setNetworkSlice(networkSlice);
    toggleEditNetworkSliceModal();
  }

  const getEditButton = (networkSlice: NetworkSlice) => {
    return <Button
      appearance=""
      onClick={() => { handleEditButton(networkSlice) }}
      className="u-no-margin--bottom">
      Edit
    </Button>
  }

  const getDeleteButton = (sliceName: string, deviceGroups: string[] | undefined) => {
    if (deviceGroups &&
      deviceGroups.length > 0) {
      return <ConfirmationButton
        appearance="negative"
        className="u-no-margin--bottom"
        confirmationModalProps={{
          title: "Warning",
          confirmButtonLabel: "Delete",
          buttonRow: (null),
          onConfirm: () => { },
          children: (
            <p>
              Network slice <b>{sliceName}</b> cannot be deleted.<br />
              Please remove the following device groups first:
              <br />
              {deviceGroups.join(", ")}.
            </p>
          ),
        }} >
        Delete
      </ConfirmationButton>
    }
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(sliceName),
        children: (
          <p>
            This will permanently delete the network slice <b>{sliceName}</b><br />
            This action cannot be undone.
          </p>
        ),
      }} >
      Delete
    </ConfirmationButton>
  }

  if (loading) {
    return <Loader text="Loading..." />;
  }

  return (
    <>
      {networkSlices.length > 0 && (
        <PageHeader title={`Network slices (${networkSlices.length})`}>
          <Button appearance="positive" onClick={toggleCreateNetworkSliceModal}>
            Create
          </Button>
        </PageHeader>
      )}
      <PageContent>
        {networkSlices.length === 0 && <NetworkSliceEmptyState />}
        {networkSlices.length > 0 && (
          <>
            {networkSlices.map((slice) => (
              <Card key={slice["name"]}>
                <h2 className="p-heading--5">{slice["name"]}</h2>
                <NetworkSliceTable slice={slice} />
                <hr />
                <div className="u-align--right">
                  {getEditButton(slice)}
                  {getDeleteButton(slice["name"], slice["profiles"])}
                </div>
              </Card>
            ))}
          </>
        )}
      </PageContent>
      {isCreateModalVisible && (
        <NetworkSliceModal
          toggleModal={toggleCreateNetworkSliceModal}
        />
      )}
      {isEditModalVisible && (
        <NetworkSliceModal
          networkSlice={networkSlice}
          toggleModal={toggleEditNetworkSliceModal}
        />
      )}
    </>
  );
};
export default NetworkConfiguration;
