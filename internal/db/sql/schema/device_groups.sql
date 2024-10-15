-- Device groups table
CREATE TABLE IF NOT EXISTS device_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    site_info TEXT NOT NULL,
    ip_domain_name TEXT NOT NULL,
    dnn TEXT NOT NULL,
    ue_ip_pool TEXT NOT NULL,
    dns_primary TEXT NOT NULL,
    mtu INTEGER NOT NULL,
    dnn_mbr_uplink INTEGER NOT NULL,
    dnn_mbr_downlink INTEGER NOT NULL,
    traffic_class_name TEXT NOT NULL,
    traffic_class_arp INTEGER NOT NULL,
    traffic_class_pdb INTEGER NOT NULL,
    traffic_class_pelr INTEGER NOT NULL,
    traffic_class_qci INTEGER NOT NULL,
    network_slice_id INTEGER,
    FOREIGN KEY (network_slice_id) REFERENCES network_slices(id) ON DELETE SET NULL
);