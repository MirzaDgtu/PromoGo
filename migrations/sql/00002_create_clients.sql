-- +goose Up
CREATE TABLE clients (
    id         BIGSERIAL PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES stores (id),
    phone      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, phone)
);

-- +goose Down
DROP TABLE clients;
