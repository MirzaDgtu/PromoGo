-- +goose Up
CREATE TABLE balances (
    client_id BIGINT PRIMARY KEY REFERENCES clients (id),
    points    BIGINT NOT NULL DEFAULT 0 CHECK (points >= 0)
);

-- +goose Down
DROP TABLE balances;
