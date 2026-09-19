-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS urls (
    uuid         UUID         NOT NULL DEFAULT gen_random_uuid(),
    short_url    VARCHAR(255) NOT NULL UNIQUE,
    original_url TEXT         NOT NULL UNIQUE,
    user_id      TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS urls;
-- +goose StatementEnd
