-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orders (
                                      id SERIAL PRIMARY KEY,
                                      number TEXT UNIQUE NOT NULL,
                                      user_id INT REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'NEW',
    accrual FLOAT DEFAULT 0,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF NOT EXISTS orders;
-- +goose StatementEnd
