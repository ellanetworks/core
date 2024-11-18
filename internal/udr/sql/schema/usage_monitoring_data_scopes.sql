CREATE TABLE IF NOT EXISTS usage_mon_data_scopes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,         -- Unique identifier for each scope
    usage_mon_data_id INTEGER NOT NULL,           -- Foreign key to link to usage_mon_data
    snssai_sd TEXT NOT NULL,                               -- SD from Snssai
    snssai_sst INTEGER NOT NULL,                           -- SST from Snssai
    dnn TEXT NOT NULL,                                     -- Individual DNN (each row represents one DNN)
    FOREIGN KEY (usage_mon_data_id) REFERENCES usage_mon_data (id) ON DELETE CASCADE
);
