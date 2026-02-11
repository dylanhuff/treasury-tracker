-- name: CreateHolding :one
INSERT INTO holdings (
    user_id,
    term,
    amount,
    yield_at_purchase,
    purchase_date,
    remaining_amount,
    face_value,
    purchase_price,
    security_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetHoldingsByUser :many
SELECT * FROM holdings
WHERE user_id = $1
ORDER BY purchase_date DESC;

-- name: GetActiveHoldingsByUser :many
SELECT * FROM holdings
WHERE user_id = $1 AND remaining_amount > 0
ORDER BY purchase_date DESC;

-- name: GetHoldingByID :one
SELECT * FROM holdings
WHERE id = $1;

-- name: UpdateHoldingRemainingAmount :one
UPDATE holdings
SET remaining_amount = $2
WHERE id = $1
RETURNING *;
