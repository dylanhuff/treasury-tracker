-- name: CreateTransaction :one
INSERT INTO transactions (
    user_id,
    type,
    term,
    amount,
    yield_at_transaction,
    balance_after,
    holding_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetTransactionsByUser :many
SELECT * FROM transactions
WHERE user_id = $1
ORDER BY timestamp DESC;

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = $1;
