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

-- name: GetDistinctYieldYears :many
SELECT DISTINCT EXTRACT(YEAR FROM date)::int AS year
FROM treasury_yields
ORDER BY year;

-- name: DeleteNonWeeklySamples :exec
DELETE FROM treasury_yields outer_ty
WHERE outer_ty.date < $1
  AND outer_ty.date NOT IN (
    SELECT DISTINCT ON (date_trunc('week', ty.date)) ty.date
    FROM treasury_yields ty
    WHERE ty.date < $1
    ORDER BY date_trunc('week', ty.date), ty.date DESC
  );

-- name: DeleteNonMonthlySamples :exec
DELETE FROM treasury_yields outer_ty
WHERE outer_ty.date < $1
  AND outer_ty.date NOT IN (
    SELECT DISTINCT ON (date_trunc('month', ty.date)) ty.date
    FROM treasury_yields ty
    WHERE ty.date < $1
    ORDER BY date_trunc('month', ty.date), ty.date DESC
  );
