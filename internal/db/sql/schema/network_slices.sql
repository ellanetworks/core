-- Network slices table
CREATE TABLE IF NOT EXISTS network_slices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sst INTEGER NOT NULL,
    sd TEXT NOT NULL,
    site_name TEXT NOT NULL,
    mcc TEXT NOT NULL,
    mnc TEXT NOT NULL
);
