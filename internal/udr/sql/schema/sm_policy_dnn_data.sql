CREATE TABLE IF NOT EXISTS sm_policy_dnn_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,       -- Unique ID
    sm_policy_snssai_data_id INTEGER NOT NULL,  -- Foreign key to sm_policy_snssai_data
    dnn TEXT NOT NULL,                          -- Data Network Name
    FOREIGN KEY (sm_policy_snssai_data_id) REFERENCES sm_policy_snssai_data (id) ON DELETE CASCADE
);
