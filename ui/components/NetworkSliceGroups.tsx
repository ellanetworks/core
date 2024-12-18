"use client";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import EditOutlinedIcon from "@mui/icons-material/EditOutlined";

import React, { useState } from "react";
import { Button, Col, ConfirmationButton, MainTable, Row, } from "@canonical/react-components";
import ProfileModal from "@/components/ProfileModal";
import { getProfilesFromNetworkSlice } from "@/utils/getProfile";
import { deleteProfile } from "@/utils/deleteProfile";
import { queryKeys } from "@/utils/queryKeys";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { NetworkSlice } from "@/components/types";
import Loader from "@/components/Loader";

type NetworkSliceTableProps = {
  slice: NetworkSlice;
  isExpanded: boolean;
};

export const NetworkSliceGroups: React.FC<NetworkSliceTableProps> = ({
  slice, isExpanded
}) => {
  const queryClient = useQueryClient();
  const [modal, setIsModalVisible] = useState({
    id: null,
    active: false,
  });
  const toggleModal = () => {
    setIsModalVisible((prevModal) => ({
      ...prevModal,
      active: !prevModal.active,
    }));
  };
  const showModal = (id: any) => {
    setIsModalVisible(() => ({
      id,
      active: true,
    }));
  };
  const { data: deviceGroupContent = [], isLoading } = useQuery({
    queryKey: [queryKeys.deviceGroups, slice["name"], slice["profiles"]?.join(",")],
    queryFn: () => getProfilesFromNetworkSlice(slice),
    enabled: isExpanded,
  });

  const handleConfirmDelete = async (name: string, networkSliceName: string) => {
    await deleteProfile({
      name,
      networkSliceName,
    });
    await queryClient.invalidateQueries({ queryKey: [queryKeys.networkSlices] });
  };

  const handleProfileEdited = async () => {
    await queryClient.invalidateQueries({
      queryKey: [queryKeys.deviceGroups],
    });
  };

  const [apiError, setApiError] = useState<string | null>(null);

  if (isLoading) {
    return <Loader text="Loading..." />;
  }

  if (!deviceGroupContent || deviceGroupContent.length === 0) {
    return <span className="u-text--muted">No device groups found.</span>;
  }

  const getDeleteButton = (deviceGroupName: string, subscribers: string[], sliceName: string) => {
    const deleteIcon = <DeleteOutlinedIcon
      className="device-group-action-button"
    />
    if (subscribers &&
      subscribers.length > 0) {
      return (
        <ConfirmationButton
          appearance="base"
          className="u-no-margin--bottom is-small"
          title="Delete device group"
          confirmationModalProps={{
            title: "Warning",
            confirmButtonLabel: "Delete",
            buttonRow: (null),
            onConfirm: () => { },
            children: (
              <p>
                Device group <b>{deviceGroupName}</b> cannot be deleted.
                <br />
                Please remove the following subscribers first:
                <br />
                {subscribers.join(", ")}.
              </p>
            ),
          }}
        >
          {deleteIcon}
        </ConfirmationButton>
      )
    }
    return (
      <ConfirmationButton
        appearance="base"
        className="u-no-margin--bottom is-small"
        shiftClickEnabled
        showShiftClickHint
        title="Delete device group"
        confirmationModalProps={{
          title: "Confirm Delete",
          confirmButtonLabel: "Delete",
          onConfirm: () => handleConfirmDelete(deviceGroupName, sliceName),
          children: (
            <p>
              This will permanently delete the device group <b>{deviceGroupName}</b>.
              <br />
              You cannot undo this action.
            </p>
          ),
        }}
      >
        {deleteIcon}
      </ConfirmationButton>
    )
  }

  const getEditButton = (modal_id: number) => {
    const editIcon = <EditOutlinedIcon
      className="device-group-action-button"
    />
    return (
      <Button
        className="u-no-margin--bottom is-small"
        small
        hasIcon
        appearance={"base"}
        onClick={() => showModal(modal_id)}
        title="Edit"
      >
        {editIcon}
      </Button>
    )
  }

  return deviceGroupContent.map((deviceGroup, deviceGroup_id) => (
    <Row key={deviceGroup["group-name"]}>
      <Col size={8}>
        {modal.active && modal.id === deviceGroup_id && (
          <ProfileModal
            toggleModal={toggleModal}
            onProfileAction={handleProfileEdited}
            deviceGroup={deviceGroup}
          />
        )}
        <MainTable
          headers={[
            {
              content: deviceGroup?.["group-name"] || "N/A",
            },
            {
              content:
                (<div className="u-align--right">
                  {getEditButton(deviceGroup_id)}
                  {getDeleteButton(deviceGroup?.["group-name"], deviceGroup?.["imsis"], slice["name"])}
                </div>
                ),
              className: "u-align--right",
            },
          ]}
          rows={[
            {
              columns: [
                { content: "Subscriber IP Pool" },
                {
                  content:
                    deviceGroup?.["ip-domain-expanded"]?.[
                    "ue-ip-pool"
                    ] || "N/A",

                  className: "u-align--right",
                },
              ],
            },
            {
              columns: [
                { content: "DNS" },
                {
                  content:
                    deviceGroup?.["ip-domain-expanded"]?.[
                    "dns-primary"
                    ] || "N/A",

                  className: "u-align--right",
                },
              ],
            },
            {
              columns: [
                { content: "MTU" },
                {
                  content:
                    deviceGroup?.["ip-domain-expanded"]?.[
                    "mtu"
                    ] || "N/A",

                  className: "u-align--right",
                },
              ],
            },
            {
              columns: [
                {
                  content: "Maximum Bitrate - Downstream",
                },
                {
                  content: deviceGroup?.[
                    "ip-domain-expanded"
                  ]?.["ue-dnn-qos"]?.["bitrate-downlink"]
                    ? `${deviceGroup?.["ip-domain-expanded"]?.[
                    "ue-dnn-qos"
                    ]?.["bitrate-downlink"] / 1_000_000
                    } Mbps`
                    : "N/A",
                  className: "u-align--right",
                },
              ],
            },
            {
              columns: [
                {
                  content: "Maximum Bitrate - Upstream",
                },
                {
                  content: deviceGroup?.[
                    "ip-domain-expanded"
                  ]?.["ue-dnn-qos"]?.["bitrate-uplink"]
                    ? `${deviceGroup?.["ip-domain-expanded"]?.[
                    "ue-dnn-qos"
                    ]?.["bitrate-uplink"] / 1_000_000
                    } Mbps`
                    : "N/A",
                  className: "u-align--right",
                },
              ],
            },
          ]}
        />
      </Col>
    </Row>
  ));
};
