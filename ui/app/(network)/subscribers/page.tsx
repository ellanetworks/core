"use client";

import React, { useState } from "react";
import {
  Button,
  MainTable,
  ConfirmationButton,
} from "@canonical/react-components";
import SubscriberModal from "@/components/SubscriberModal";
import { getSubscribers } from "@/utils/getSubscribers";
import { getProfiles } from "@/utils/getProfile";
import { getNetworkSlices } from "@/utils/getNetworkSlices";
import SyncOutlinedIcon from "@mui/icons-material/SyncOutlined";
import { deleteSubscriber } from "@/utils/deleteSubscriber";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";

export type Subscriber = {
  plmnID: string;
  ueId: string;
};

const Subscribers = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);
  const [isEditModalVisible, setEditModalVisible] = useState(false);
  const [subscriber, setSubscriber] = useState<any | undefined>(undefined);

  const { data: subscribers = [], isLoading: isSubscribersLoading } = useQuery({
    queryKey: [queryKeys.subscribers],
    queryFn: getSubscribers,
  });

  const { data: deviceGroups = [], isLoading: isProfilesLoading } = useQuery({
    queryKey: [queryKeys.deviceGroups],
    queryFn: getProfiles,
  });

  const { data: slices = [], isLoading: isSlicesLoading } = useQuery({
    queryKey: [queryKeys.networkSlices],
    queryFn: getNetworkSlices,
  });

  const handleRefresh = async () => {
    await queryClient.invalidateQueries({ queryKey: [queryKeys.subscribers] });
    await queryClient.invalidateQueries({ queryKey: [queryKeys.deviceGroups] });
    await queryClient.invalidateQueries({ queryKey: [queryKeys.networkSlices] });
  };

  const handleConfirmDelete = async (subscriber: string) => {
    await deleteSubscriber(subscriber);
    await handleRefresh();
  };

  const toggleCreateModal = () => setCreateModalVisible((prev) => !prev);
  const toggleEditModal = () => setEditModalVisible((prev) => !prev);

  const handleEditButton = (subscriber: any) => {
    setSubscriber(subscriber);
    toggleEditModal();
  }

  const getEditButton = (subscriber: any) => {
    return <Button
      appearance=""
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      onClick={() => { handleEditButton(subscriber) }}
    >
      Edit
    </Button>
  }

  const getDeleteButton = (imsi: string) => {
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(imsi),
        children: (
          <p>
            This will permanently delete the subscriber{" "}
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

  const tableContent = subscribers.map((subscriber) => {
    const rawIMSI = subscriber.ueId.split("-")[1];
    return {
      key: rawIMSI,
      columns: [
        { content: rawIMSI },
        {
          content: (
            <div className="u-align--right">
              {getEditButton(subscriber)}
              {getDeleteButton(rawIMSI)}
            </div>
          ),
        },
      ],
    };
  });

  if (isSubscribersLoading || isProfilesLoading || isSlicesLoading) {
    return <Loader text="Loading..." />;
  }

  return (
    <>
      <PageHeader title={`Subscribers (${subscribers.length})`}>
        <Button
          hasIcon
          appearance="base"
          onClick={handleRefresh}
          title="refresh subscriber list"
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
            { content: "IMSI" },
            { content: "Actions", className: "u-align--right" },
          ]}
          rows={tableContent}
        />
      </PageContent>
      {isCreateModalVisible && <SubscriberModal toggleModal={toggleCreateModal} slices={slices} deviceGroups={deviceGroups} />}
      {isEditModalVisible &&
        <SubscriberModal toggleModal={toggleEditModal} subscriber={subscriber} slices={slices} deviceGroups={deviceGroups} />}
    </>
  );
};
export default Subscribers;
