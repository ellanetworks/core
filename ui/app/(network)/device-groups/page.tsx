"use client";

import React, { useState } from "react";
import {
  Button,
  MainTable,
  ConfirmationButton,
} from "@canonical/react-components";
import DeviceGroupModal from "@/components/DeviceGroupModal";
import { listDeviceGroups } from "@/queries/deviceGroups";
import SyncOutlinedIcon from "@mui/icons-material/SyncOutlined";
import { deleteDeviceGroup } from "@/queries/deviceGroups";
import Loader from "@/components/Loader";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/utils/queryKeys";
import PageHeader from "@/components/PageHeader";
import PageContent from "@/components/PageContent";

const DeviceGroups = () => {
  const queryClient = useQueryClient();
  const [isCreateModalVisible, setCreateModalVisible] = useState(false);

  const { data: device_groups = [], isLoading: isDeviceGroupsLoading } = useQuery({
    queryKey: [queryKeys.deviceGroups],
    queryFn: listDeviceGroups,
  });


  const handleRefresh = async () => {
    await queryClient.invalidateQueries({ queryKey: [queryKeys.deviceGroups] });
  };

  const handleConfirmDelete = async (id: string) => {
    await deleteDeviceGroup(id);
    await handleRefresh();
  };

  const toggleCreateModal = () => setCreateModalVisible((prev) => !prev);

  const getDeleteButton = (name: string, device_group_id: string) => {
    return <ConfirmationButton
      appearance="negative"
      className="u-no-margin--bottom"
      shiftClickEnabled
      showShiftClickHint
      confirmationModalProps={{
        title: "Confirm Delete",
        confirmButtonLabel: "Delete",
        onConfirm: () => handleConfirmDelete(device_group_id),
        children: (
          <p>
            This will permanently delete the device group{" "}
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
  const tableContent = device_groups.map((device_group) => {
    return {
      key: device_group.name,
      columns: [
        { content: device_group?.["id"] },
        { content: device_group?.["name"] },
        { content: device_group?.["ue_ip_pool"] },
        { content: device_group?.["dns_primary"] },
        { content: device_group?.["mtu"] },
        { content: device_group?.["dnn_mbr_downlink"] / 1000000 },
        { content: device_group?.["dnn_mbr_uplink"] / 1000000 },
        { content: device_group?.["network_slice_id"] },
        {
          content: (
            <div className="u-align--right">
              {getDeleteButton(device_group.name, device_group.id)}
            </div>
          ),
        },
      ],
    };
  });

  if (isDeviceGroupsLoading) {
    return <Loader text="Loading..." />;
  }

  return (
    <>
      <PageHeader title={`Device Groups (${device_groups.length})`}>
        <Button
          hasIcon
          appearance="base"
          onClick={handleRefresh}
          title="refresh device groups"
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
            { content: "IP Pool" },
            { content: "DNS" },
            { content: "MTU" },
            { content: "Downlink (Mbps)" },
            { content: "Uplink (Mbps)" },
            { content: "Network Slice ID" },
            { content: "Actions", className: "u-align--right" },
          ]}
          rows={tableContent}
        />
      </PageContent>
      {isCreateModalVisible && <DeviceGroupModal toggleModal={toggleCreateModal} />}
    </>
  );
};
export default DeviceGroups;
