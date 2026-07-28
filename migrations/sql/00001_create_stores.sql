-- +goose Up
CREATE TABLE stores (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL,
    api_key_hash TEXT NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE stores;
