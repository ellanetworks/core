// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Button,
  Chip,
  IconButton,
  Link,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
} from "@mui/icons-material";
import type { OperatorIMS } from "@/queries/operator";
import {
  deleteIMSApplicationServer,
  deleteIMSService,
  listIMSApplicationServers,
  listIMSServices,
  type IMSApplicationServer,
  type IMSService,
} from "@/queries/ims";
import EditOperatorIMSModal from "@/components/EditOperatorIMSModal";
import IMSApplicationServerModal from "@/components/IMSApplicationServerModal";
import MCPTTServiceModal from "@/components/MCPTTServiceModal";
import IMSServiceFiltersModal from "@/components/IMSServiceFiltersModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { IMS_DATA_NETWORK } from "@/utils/voice";

interface IMSSectionProps {
  ims?: OperatorIMS;
  canEdit: boolean;
  onModalOpenChange: (open: boolean) => void;
}

const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : "Unknown error occurred.";

const IMSSection: React.FC<IMSSectionProps> = ({
  ims,
  canEdit,
  onModalOpenChange,
}) => {
  const theme = useTheme();
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [asModal, setASModal] = useState<{
    applicationServer?: IMSApplicationServer;
  } | null>(null);
  const [asToDelete, setASToDelete] = useState<IMSApplicationServer | null>(
    null,
  );
  const [serviceModal, setServiceModal] = useState<{
    service?: IMSService;
  } | null>(null);
  const [serviceToDelete, setServiceToDelete] = useState<IMSService | null>(
    null,
  );
  const [serviceToView, setServiceToView] = useState<IMSService | null>(null);

  const anyModalOpen =
    editOpen ||
    asModal !== null ||
    asToDelete !== null ||
    serviceModal !== null ||
    serviceToDelete !== null ||
    serviceToView !== null;

  useEffect(() => {
    onModalOpenChange(anyModalOpen);
  }, [anyModalOpen, onModalOpenChange]);

  const serversQuery = useQuery<IMSApplicationServer[]>({
    queryKey: ["ims-application-servers"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listIMSApplicationServers(accessToken!),
    placeholderData: (prev) => prev,
  });

  const servicesQuery = useQuery<IMSService[]>({
    queryKey: ["ims-services"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listIMSServices(accessToken!),
    placeholderData: (prev) => prev,
  });

  const servers = serversQuery.data ?? [];
  const services = servicesQuery.data ?? [];
  const serversLoading = serversQuery.isLoading && !serversQuery.data;
  const servicesLoading = servicesQuery.isLoading && !servicesQuery.data;

  const addresses = ims?.pcscfAddresses ?? [];

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["ims-application-servers"] });
    queryClient.invalidateQueries({ queryKey: ["ims-services"] });
  };

  const handleDeleteAS = async () => {
    if (!asToDelete || !accessToken) return;
    try {
      await deleteIMSApplicationServer(accessToken, asToDelete.name);
      refresh();
      showSnackbar("Application server deleted successfully.", "success");
    } catch (error) {
      showSnackbar(
        `Failed to delete application server: ${errorMessage(error)}`,
        "error",
      );
    } finally {
      setASToDelete(null);
    }
  };

  const handleDeleteService = async () => {
    if (!serviceToDelete || !accessToken) return;
    try {
      await deleteIMSService(accessToken, serviceToDelete.name);
      refresh();
      showSnackbar("Service deleted successfully.", "success");
    } catch (error) {
      showSnackbar(`Failed to delete service: ${errorMessage(error)}`, "error");
    } finally {
      setServiceToDelete(null);
    }
  };

  const headerRowSx = {
    "& .MuiTableRow-root:first-of-type .MuiTableCell-root": {
      fontWeight: 600,
      backgroundColor: theme.palette.backgroundSubtle,
    },
  };

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        IMS
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let 4G and 5G subscribers make voice and video calls through your IMS.
        Each subscriber needs a phone number and a policy on the{" "}
        {IMS_DATA_NETWORK} data network, marked as IMS.
      </Typography>

      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
        <Table>
          <TableBody>
            <TableRow>
              <TableCell sx={{ fontWeight: 600, width: "35%" }}>
                <Tooltip title="The IMS entry points handsets call" arrow>
                  <span>P-CSCF Addresses</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ width: "55%", wordBreak: "break-word" }}>
                {addresses.length > 0 ? addresses.join(", ") : "N/A"}
              </TableCell>
              <TableCell sx={{ width: "10%", textAlign: "right" }}>
                {canEdit && (
                  <Tooltip title="Edit P-CSCF addresses" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setEditOpen(true)}
                      aria-label="Edit P-CSCF addresses"
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Application Servers
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          SIP servers that services send requests to.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setASModal({})}
            aria-label="Add application server"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add Application Server
          </Button>
        )}
      </Stack>
      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
        <Table sx={{ tableLayout: "fixed" }}>
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "20%" }}>Name</TableCell>
              <TableCell sx={{ width: "30%" }}>Server Name</TableCell>
              <TableCell>Public Service Identities</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {serversLoading && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!serversLoading && servers.length === 0 && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Typography variant="body2" color="textSecondary">
                    No application servers yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {servers.map((as) => (
              <TableRow key={as.name}>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {as.name}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {as.server_name}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {as.public_service_identities.length > 0
                    ? as.public_service_identities.join(", ")
                    : "None"}
                </TableCell>
                <TableCell align="right">
                  {canEdit && (
                    <>
                      <Tooltip title="Edit application server" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setASModal({ applicationServer: as })}
                          aria-label={`Edit application server ${as.name}`}
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete application server" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setASToDelete(as)}
                          aria-label={`Delete application server ${as.name}`}
                        >
                          <DeleteIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    </>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Services
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          Services decide which requests go to which application server. Assign
          them to profiles. Custom services are managed through the API.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setServiceModal({})}
            aria-label="Add MCPTT service"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add MCPTT Service
          </Button>
        )}
      </Stack>
      <TableContainer sx={TABLE_CONTAINER_SX}>
        <Table sx={{ tableLayout: "fixed" }}>
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "25%" }}>Name</TableCell>
              <TableCell sx={{ width: "15%" }}>Kind</TableCell>
              <TableCell sx={{ width: "25%" }}>Application Server</TableCell>
              <TableCell>Filter Criteria</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {servicesLoading && (
              <TableRow>
                <TableCell colSpan={5}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!servicesLoading && services.length === 0 && (
              <TableRow>
                <TableCell colSpan={5}>
                  <Typography variant="body2" color="textSecondary">
                    No services yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {services.map((service) => {
              const count = service.initial_filter_criteria?.length ?? 0;
              return (
                <TableRow key={service.name}>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {service.name}
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={service.kind === "mcptt" ? "MCPTT" : "Custom"}
                      size="small"
                      variant="outlined"
                    />
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {service.application_server_name}
                  </TableCell>
                  <TableCell>
                    <Link
                      component="button"
                      variant="body2"
                      onClick={() => setServiceToView(service)}
                      aria-label={`View filter criteria of ${service.name}`}
                    >
                      {count === 1 ? "1 filter" : `${count} filters`}
                    </Link>
                  </TableCell>
                  <TableCell align="right">
                    {canEdit && (
                      <>
                        {service.kind === "mcptt" && (
                          <Tooltip title="Edit service" arrow>
                            <IconButton
                              size="small"
                              onClick={() => setServiceModal({ service })}
                              aria-label={`Edit service ${service.name}`}
                            >
                              <EditIcon fontSize="small" color="primary" />
                            </IconButton>
                          </Tooltip>
                        )}
                        <Tooltip title="Delete service" arrow>
                          <IconButton
                            size="small"
                            onClick={() => setServiceToDelete(service)}
                            aria-label={`Delete service ${service.name}`}
                          >
                            <DeleteIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                      </>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>

      {editOpen && (
        <EditOperatorIMSModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["operator"] });
            showSnackbar("P-CSCF addresses updated successfully.", "success");
          }}
          initialData={{ pcscfAddresses: addresses }}
        />
      )}
      {asModal && (
        <IMSApplicationServerModal
          open
          onClose={() => setASModal(null)}
          onSuccess={() => {
            refresh();
            showSnackbar(
              asModal.applicationServer
                ? "Application server updated successfully."
                : "Application server added successfully.",
              "success",
            );
          }}
          applicationServer={asModal.applicationServer}
        />
      )}
      {asToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setASToDelete(null)}
          onConfirm={handleDeleteAS}
          title="Delete Application Server"
          description={`Delete the application server ${asToDelete.name}?`}
        />
      )}
      {serviceModal && (
        <MCPTTServiceModal
          open
          onClose={() => setServiceModal(null)}
          onSuccess={() => {
            refresh();
            showSnackbar(
              serviceModal.service
                ? "Service updated successfully."
                : "Service added successfully.",
              "success",
            );
          }}
          service={serviceModal.service}
          homeDomain={ims?.homeDomain}
        />
      )}
      {serviceToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setServiceToDelete(null)}
          onConfirm={handleDeleteService}
          title="Delete Service"
          description={`Delete the service ${serviceToDelete.name}?`}
        />
      )}
      {serviceToView && (
        <IMSServiceFiltersModal
          open
          onClose={() => setServiceToView(null)}
          service={serviceToView}
        />
      )}
    </Box>
  );
};

export default IMSSection;
