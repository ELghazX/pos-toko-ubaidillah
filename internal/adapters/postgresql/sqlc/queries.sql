-- name: ListCategories :many
SELECT * FROM categories
WHERE is_active = true
ORDER BY name ASC;

-- name: GetCategoryByID :one
SELECT * FROM categories
WHERE id = $1 LIMIT 1;


-- name: CreateCategory :one
INSERT INTO categories (
	code, name
) VALUES (
	$1, $2
) RETURNING *;


-- name: UpdateCategory :exec
UPDATE categories
SET code = $2, name = $3, updated_at = CURRENT_TIMESTAMP
WHERE id  = $1;

-- name: DeleteCategory :exec
UPDATE categories
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;
