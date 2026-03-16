CREATE TABLE IF NOT EXISTS known_destinations (
    resource_id  TEXT NOT NULL,
    destination  TEXT NOT NULL,
    first_seen   TIMESTAMPTZ NOT NULL,
    last_seen    TIMESTAMPTZ NOT NULL,
    tx_count     INT NOT NULL DEFAULT 1,
    is_internal  BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (resource_id, destination)
);
