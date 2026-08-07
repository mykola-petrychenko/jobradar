CREATE TABLE IF NOT EXISTS postings (
    id         bigserial   PRIMARY KEY,
    source     text        NOT NULL,
    source_id  text        NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now(),
    raw        jsonb       NOT NULL,
    UNIQUE (source, source_id)
);