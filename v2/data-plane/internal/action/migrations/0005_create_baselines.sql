CREATE TABLE IF NOT EXISTS baselines (
    scope_type   TEXT NOT NULL,
    scope_id     TEXT NOT NULL,
    metric       TEXT NOT NULL,
    avg          DOUBLE PRECISION NOT NULL,
    stddev       DOUBLE PRECISION NOT NULL,
    p95          DOUBLE PRECISION NOT NULL,
    sample_size  INT NOT NULL,
    window_days  INT NOT NULL,
    computed_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope_type, scope_id, metric)
);
