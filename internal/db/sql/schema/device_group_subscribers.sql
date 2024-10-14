CREATE TABLE IF NOT EXISTS device_group_subscribers (
    device_group_id INTEGER NOT NULL,
    subscriber_id INTEGER UNIQUE NOT NULL,
    FOREIGN KEY (device_group_id) REFERENCES device_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
    PRIMARY KEY (device_group_id, subscriber_id)
);