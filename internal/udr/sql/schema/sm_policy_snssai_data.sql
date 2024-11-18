CREATE TABLE IF NOT EXISTS sm_policy_snssai_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sm_policy_data_id INTEGER NOT NULL, 
    snssai_sd TEXT NOT NULL, 
    snssai_sst INTEGER NOT NULL, 
    FOREIGN KEY (sm_policy_data_id) REFERENCES sm_policy_data (id) ON DELETE CASCADE
);
