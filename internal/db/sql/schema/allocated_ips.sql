CREATE TABLE AllocatedIP (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    imsi TEXT NOT NULL UNIQUE, -- Unique IMSI
    ip_address TEXT NOT NULL,
    pool_id INTEGER NOT NULL,
    allocated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (pool_id) REFERENCES IPPool(id) ON DELETE CASCADE,
    UNIQUE(imsi, pool_id),
    UNIQUE(ip_address, pool_id)
);