-- +goose Up
-- +goose StatementBegin
ALTER TABLE urls ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE urls DROP COLUMN IF EXISTS user_id;
-- +goose StatementEnd
