package shortener

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func isSlugUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" &&
		pgErr.ConstraintName == "links_slug_unique"
}
