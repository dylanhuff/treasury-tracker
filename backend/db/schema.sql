-- Treasury Tracker - Database Schema
-- Manages users, transactions, and treasury security holdings.

DROP TABLE IF EXISTS holdings CASCADE;
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TYPE IF EXISTS transaction_type CASCADE;

CREATE TYPE transaction_type AS ENUM ('fund', 'withdraw', 'buy', 'sell');

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    balance NUMERIC(12, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_balance_non_negative CHECK (balance >= 0)
);

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    type transaction_type NOT NULL,
    term VARCHAR(10),
    amount DECIMAL(12, 2) NOT NULL,
    yield_at_transaction DECIMAL(5, 2),
    balance_after DECIMAL(12, 2) NOT NULL,
    holding_id INTEGER,

    CONSTRAINT transactions_amount_positive CHECK (amount > 0)
);

CREATE TABLE holdings (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term VARCHAR(10) NOT NULL,
    amount DECIMAL(12, 2) NOT NULL,
    yield_at_purchase DECIMAL(5, 2) NOT NULL,
    purchase_date TIMESTAMP NOT NULL DEFAULT NOW(),
    remaining_amount DECIMAL(12, 2) NOT NULL,
    face_value DECIMAL(12, 2),
    purchase_price DECIMAL(12, 2),
    security_type VARCHAR(10),

    CONSTRAINT holdings_amount_positive CHECK (amount > 0),
    CONSTRAINT holdings_remaining_non_negative CHECK (remaining_amount >= 0),
    CONSTRAINT holdings_remaining_lte_amount CHECK (remaining_amount <= amount)
);

CREATE INDEX idx_users_name ON users(name);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_timestamp ON transactions(timestamp DESC);
CREATE INDEX idx_transactions_type ON transactions(type);
CREATE INDEX idx_holdings_user_id ON holdings(user_id);
CREATE INDEX idx_holdings_purchase_date ON holdings(purchase_date DESC);

COMMENT ON TABLE users IS 'User accounts with current balance';
COMMENT ON TABLE transactions IS 'All financial transactions (deposits, withdrawals, treasury trades)';
COMMENT ON TABLE holdings IS 'Active treasury holdings (bills, notes, bonds)';
COMMENT ON COLUMN holdings.security_type IS 'Type of treasury security: bill, note, or bond';
COMMENT ON COLUMN holdings.face_value IS 'Amount received at maturity';
COMMENT ON COLUMN holdings.purchase_price IS 'Actual price paid (discounted for T-Bills)';
COMMENT ON COLUMN transactions.holding_id IS 'References the holding for buy/sell transactions';

CREATE TABLE treasury_yields (
    date DATE PRIMARY KEY,
    bc_1month NUMERIC(6, 3),
    bc_3month NUMERIC(6, 3),
    bc_6month NUMERIC(6, 3),
    bc_1year  NUMERIC(6, 3),
    bc_2year  NUMERIC(6, 3),
    bc_5year  NUMERIC(6, 3),
    bc_10year NUMERIC(6, 3),
    bc_30year NUMERIC(6, 3)
);

COMMENT ON TABLE treasury_yields IS 'Daily treasury yield curve data from treasury.gov';
