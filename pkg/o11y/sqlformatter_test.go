package o11y

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// The JSON helpers read the same documents the same way on each engine.
func TestJSONFormatterOnEveryEngine(t *testing.T) {
	for _, e := range engines() {
		t.Run(e.name, func(t *testing.T) {
			ctx := context.Background()
			store, _ := e.open(t)
			db := store.SQLDB()
			_, err := db.ExecContext(ctx, `CREATE TABLE doc (id INTEGER PRIMARY KEY, data TEXT NOT NULL)`)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `INSERT INTO doc (id, data) VALUES (1, '{"spec":{"display":{"name":"cpu"}},"list":[1,2],"tags":["a","b"],"keys":{"x":1,"y":2}}')`)
			require.NoError(t, err)
			f := store.Formatter()

			var name string
			require.NoError(t, store.BunDB().NewRaw("SELECT "+string(f.JSONExtractString("data", "$.spec.display.name"))+" FROM doc").Scan(ctx, &name))
			require.Equal(t, "cpu", name)

			var n int
			require.NoError(t, store.BunDB().NewRaw("SELECT COUNT(*) FROM doc WHERE "+string(f.JSONIsArray("data", "$.list"))).Scan(ctx, &n))
			require.Equal(t, 1, n)
			require.NoError(t, store.BunDB().NewRaw("SELECT COUNT(*) FROM doc WHERE "+string(f.JSONIsArray("data", "$.spec"))).Scan(ctx, &n))
			require.Equal(t, 0, n)

			from, col := f.JSONArrayOfStrings("data", "$.tags", "t")
			require.NoError(t, store.BunDB().NewRaw("SELECT COUNT(*) FROM doc, "+string(from)+" WHERE "+string(col)+" = 'b'").Scan(ctx, &n))
			require.Equal(t, 1, n)

			from, col = f.JSONKeys("data", "$.keys", "k")
			require.NoError(t, store.BunDB().NewRaw("SELECT COUNT(*) FROM doc, "+string(from)+" WHERE "+string(col)+" IN ('x', 'y')").Scan(ctx, &n))
			require.Equal(t, 2, n)

			var lit string
			require.NoError(t, store.BunDB().NewRaw("SELECT "+string(f.JSONArrayLiteral("a", "b"))).Scan(ctx, &lit))
			require.JSONEq(t, `["a","b"]`, lit)
		})
	}
}
