CREATE TABLE IF NOT EXISTS network_slice_device_groups (
    network_slice_id INTEGER NOT NULL,
    device_group_id INTEGER UNIQUE NOT NULL,
    FOREIGN KEY (network_slice_id) REFERENCES network_slices(id) ON DELETE CASCADE,
    FOREIGN KEY (device_group_id) REFERENCES device_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (network_slice_id, device_group_id)
);