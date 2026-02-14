-- name: UpsertTreasuryYield :exec
INSERT INTO treasury_yields (
    date, bc_1month, bc_3month, bc_6month, bc_1year,
    bc_2year, bc_5year, bc_10year, bc_30year
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) ON CONFLICT (date) DO NOTHING;

-- name: GetYieldsByDateRange :many
SELECT * FROM treasury_yields
WHERE date >= $1 AND date <= $2
ORDER BY date ASC;

-- name: GetLatestYield :one
SELECT * FROM treasury_yields
ORDER BY date DESC
LIMIT 1;

-- name: GetMaxYieldDate :one
SELECT COALESCE(MAX(date), '1900-01-01'::date) AS max_date
FROM treasury_yields;
