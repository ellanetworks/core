CREATE TABLE IF NOT EXISTS network_slice_gnbs (
    network_slice_id INTEGER NOT NULL,
    gnb_id INTEGER UNIQUE NOT NULL,
    FOREIGN KEY (network_slice_id) REFERENCES network_slices(id) ON DELETE CASCADE,
    FOREIGN KEY (gnb_id) REFERENCES gnbs(id) ON DELETE CASCADE,
    PRIMARY KEY (network_slice_id, gnb_id)
);