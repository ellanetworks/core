"use client";
import AddOutlinedIcon from "@mui/icons-material/AddOutlined";
import ExpandMoreOutlinedIcon from "@mui/icons-material/ExpandMoreOutlined";

import React, { useState } from "react";
import { Button, MainTable } from "@canonical/react-components";
import DeviceGroupModal from "@/components/DeviceGroupModal";
import { queryKeys } from "@/utils/queryKeys";
import { useQueryClient } from "@tanstack/react-query";
import { NetworkSlice } from "@/components/types";
// import { NetworkSliceGroups } from "@/components/NetworkSliceGroups";

type NetworkSliceTableProps = {
  slice: NetworkSlice;
};

// export const NetworkSliceTable: React.FC<NetworkSliceTableProps> = ({
//   slice,
// }) => {
//   const queryClient = useQueryClient();
//   const [isExpanded, setExpanded] = useState(false);
//   const [isModalVisible, setIsModalVisible] = useState(false);
//   const toggleModal = () => setIsModalVisible(!isModalVisible);

//   const handleDeviceGroupCreated = async () => {
//     await queryClient.invalidateQueries({
//       queryKey: [queryKeys.networkSlices],
//     });
//     await queryClient.invalidateQueries({
//       queryKey: [queryKeys.deviceGroups, slice["slice-name"]],
//     });
//   };

//   return (
//     <>
//       {isModalVisible && slice?.["slice-name"] && (
//         <DeviceGroupModal
//           toggleModal={toggleModal}
//           onDeviceGroupAction={handleDeviceGroupCreated}
//           networkSliceName={slice["slice-name"]}
//         />
//       )}
//       <MainTable
//         expanding
//         headers={[
//           {
//             content: "Key",
//           },
//           {
//             content: "Value",
//             className: "u-align--right",
//           },
//         ]}
//         rows={[
//           {
//             columns: [
//               { content: "MCC" },
//               {
//                 content: slice["site-info"]?.plmn.mcc || "N/A",
//                 className: "u-align--right",
//               },
//             ],
//             key: `mcc-${slice["slice-name"]}`,
//           },
//           {
//             columns: [
//               { content: "MNC" },
//               {
//                 content: slice["site-info"]?.plmn.mnc || "N/A",
//                 className: "u-align--right",
//               },
//             ],
//             key: `mnc-${slice["slice-name"]}`,
//           },
//           {
//             columns: [
//               { content: "UPF" },
//               {
//                 content: `${slice["site-info"]?.upf?.["upf-name"] || "N/A"}:${
//                   slice["site-info"]?.upf?.["upf-port"] || "N/A"
//                 }`,

//                 className: "u-align--right",
//               },
//             ],
//             key: `upf-${slice["slice-name"]}`,
//           },
//           {
//             columns: [
//               { content: "gNodeBs" },
//               {
//                 content:
//                   slice?.["site-info"]?.gNodeBs?.length.toString() || "N/A",
//                 className: "u-align--right",
//               },
//             ],
//             key: `gNodeBs-${slice["slice-name"]}`,
//           },
//           {
//             columns: [
//               {
//                 content: `Device Groups (${
//                   !slice["site-device-group"] ||
//                   slice["site-device-group"].length
//                 })`,
//               },
//               {
//                 content: (
//                   <div className="u-align--right">
//                     <Button
//                       className="u-toggle"
//                       small
//                       hasIcon
//                       appearance={"base"}
//                       onClick={toggleModal}
//                       title="add device group"
//                     >
//                       <AddOutlinedIcon
//                         fontSize="small"
//                         style={{ color: "#666" }}
//                       />
//                     </Button>
//                     <Button
//                       small
//                       hasIcon
//                       disabled={
//                         !slice["site-device-group"] ||
//                         slice["site-device-group"].length === 0
//                       }
//                       appearance={"base"}
//                       onClick={() => setExpanded(!isExpanded)}
//                       title="expand device groups"
//                     >
//                       <ExpandMoreOutlinedIcon
//                         fontSize="small"
//                         style={{
//                           color: "#666",
//                           transform: isExpanded
//                             ? "rotate(180deg)"
//                             : "rotate(0)",
//                         }}
//                       />
//                     </Button>
//                   </div>
//                 ),
//               },
//             ],
//             expandedContent: (
//               <NetworkSliceGroups slice={slice} isExpanded={isExpanded} />
//             ),
//             expanded: isExpanded,
//             key: `device-groups-${slice["slice-name"]}`,
//           },
//         ]}
//       />
//     </>
//   );
// };
