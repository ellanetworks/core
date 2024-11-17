-- UPFs Inventory table
CREATE TABLE IF NOT EXISTS upfs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    network_slice_id INTEGER,
    FOREIGN KEY (network_slice_id) REFERENCES network_slices(id) ON DELETE SET NULL
);