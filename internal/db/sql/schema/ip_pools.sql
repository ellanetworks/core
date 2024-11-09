CREATE TABLE IPPool (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_group_id INTEGER NOT NULL,
    cidr TEXT NOT NULL, -- E.g., "16.0.0.0/24"
    FOREIGN KEY (device_group_id) REFERENCES DeviceGroup(id) ON DELETE CASCADE
);