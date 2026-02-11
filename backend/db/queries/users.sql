-- name: ListUsers :many
SELECT id, name, balance, created_at
FROM users
ORDER BY name ASC;

-- name: GetUser :one
SELECT id, name, balance, created_at
FROM users
WHERE id = $1;

-- name: GetUserForUpdate :one
SELECT id, name, balance, created_at
FROM users
WHERE id = $1
FOR UPDATE;

-- name: CreateUser :one
INSERT INTO users (name, balance)
VALUES ($1, $2)
RETURNING id, name, balance, created_at;

-- name: UpdateUserBalance :one
UPDATE users
SET balance = balance + $1
WHERE id = $2
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
