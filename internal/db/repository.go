package db

// Repository is the hand-written extension point around sqlc-generated queries.
// Add transaction helpers and higher-level indexing operations here as the
// ingestion pipeline grows.
type Repository struct {
	*Queries
}

func NewRepository(conn DBTX) *Repository {
	return &Repository{Queries: New(conn)}
}
