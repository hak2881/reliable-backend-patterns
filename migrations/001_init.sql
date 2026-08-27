BEGIN;

CREATE TABLE ledger_entries (
    id              BIGSERIAL PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    account_id      TEXT NOT NULL,
    amount          BIGINT NOT NULL CHECK (amount <> 0),
    reference       TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ledger_entries_account_created_idx
    ON ledger_entries (account_id, created_at, id);

CREATE TABLE outbox_events (
    id            BIGSERIAL PRIMARY KEY,
    topic         TEXT NOT NULL,
    aggregate_key TEXT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ,
    UNIQUE (topic, aggregate_key)
);

COMMIT;
