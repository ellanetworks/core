"use client";

import React, { useState } from "react";
import {
  Button,
  MainTable,
  ConfirmationButton,
} from "@canonical/react-components";
import SubscriberModal from "@/components/SubscriberModal";
import { listSubscribers } from "@/queries/subscribers";
import SyncOutlinedIcon from "@mui/icons-material/SyncOutlined";
import { deleteSubscriber } from "@/queries/subscribers";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";

export type Subscriber = {
  id: string;
  imsi: string;
  plmn_id: string;
  opc: string;
  key: string;
  sequence_number: string;
  device_group_id: string;
};

const Subscribers = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);

  const { data: subscribers = [], isLoading: isSubscribersLoading } = useQuery({
    queryKey: [queryKeys.subscribers],
    queryFn: listSubscribers,
  });


  const handleRefresh = async () => {
    await queryClient.invalidateQueries({ queryKey: [queryKeys.subscribers] });
  };

  const handleConfirmDelete = async (subscriber: string) => {
    await deleteSubscriber(subscriber);
    await handleRefresh();
  };

  const toggleCreateModal = () => setCreateModalVisible((prev) => !prev);


  const getDeleteButton = (imsi: string, subscriber_id: string) => {
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(subscriber_id),
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
    return {
      key: subscriber.imsi,
      columns: [
        { content: subscriber.id },
        { content: subscriber.imsi },
        { content: subscriber.device_group_id },
        {
          content: (
            <div className="u-align--right">
              {getDeleteButton(subscriber.imsi, subscriber.id)}
            </div>
          ),
        },
      ],
    };
  });

  if (isSubscribersLoading) {
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
            { content: "Id" },
            { content: "IMSI" },
            { content: "Device Group ID" },
            { content: "Actions", className: "u-align--right" },
          ]}
          rows={tableContent}
        />
      </PageContent>
      {isCreateModalVisible && <SubscriberModal toggleModal={toggleCreateModal} />}
    </>
  );
};
export default Subscribers;
