package ctxtypes

import "context"

type ctxKey string

const (
	DatastoreContextMaxThreadsKey ctxKey = "datastore_max_threads"
)

// SetDatastoreMaxThreads stores the max threads value in context.
func SetDatastoreMaxThreads(ctx context.Context, maxThreads int) context.Context {
	return context.WithValue(ctx, DatastoreContextMaxThreadsKey, maxThreads)
}
