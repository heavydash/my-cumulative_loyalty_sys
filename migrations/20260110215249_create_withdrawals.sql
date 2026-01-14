-- +goose Up
-- +goose StatementBegin
CREATE TABLE withdrawals (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_number TEXT NOT NULL UNIQUE,
    sum NUMERIC(12,2) NOT NULL CHECK (sum > 0),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()

);

CREATE INDEX ix_withdrawals_user_id ON withdrawals(user_id);
CREATE INDEX ix_withdrawals_processed_at_desc ON withdrawals(processed_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS withdrawals CASCADE;
-- +goose StatementEnd
