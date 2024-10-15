-- Subscribers table
CREATE TABLE IF NOT EXISTS subscribers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    imsi TEXT NOT NULL UNIQUE,
    plmn_id TEXT NOT NULL,
    opc TEXT NOT NULL,
    key TEXT NOT NULL,
    sequence_number TEXT NOT NULL,
    device_group_id INTEGER,
    FOREIGN KEY (device_group_id) REFERENCES device_groups(id) ON DELETE SET NULL
);