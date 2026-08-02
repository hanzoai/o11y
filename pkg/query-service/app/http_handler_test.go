package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pipeline version is a PATH segment of /v1/o11y/logs/pipelines/{version}.
// Drive a REAL router registered at the real template: the parse is what turns
// that segment into the version the pipelines listing answers for, and reading
// the wrong half of the URL yields "" — which is not "latest" and not a number,
// so every request would fail as invalid input.
func TestParseAgentConfigVersionReadsVersionFromThePath(t *testing.T) {
	serve := func(t *testing.T, target string) (int, error) {
		t.Helper()

		var (
			version int
			err     error
		)

		router := routing.Serve(http.MethodGet, "/v1/o11y/logs/pipelines/{version}", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			version, err = parseAgentConfigVersion(r)
		}))

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
		require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the parse")

		return version, err
	}

	t.Run("numbered", func(t *testing.T) {
		version, err := serve(t, "/v1/o11y/logs/pipelines/3")
		require.NoError(t, err)
		assert.Equal(t, 3, version)
	})

	t.Run("latest", func(t *testing.T) {
		version, err := serve(t, "/v1/o11y/logs/pipelines/latest")
		require.NoError(t, err)
		assert.Equal(t, -1, version)
	})

	// A query value of the same name is a different value on a different half of
	// the URL, and the parse must not answer to it.
	t.Run("query is not the path", func(t *testing.T) {
		version, err := serve(t, "/v1/o11y/logs/pipelines/3?version=9")
		require.NoError(t, err)
		assert.Equal(t, 3, version)
	})
}

func TestPrepareQuery(t *testing.T) {
	type testCase struct {
		name        string
		postData    *model.DashboardVars
		query       string
		expectedErr bool
		errMsg      string
	}

	testCases := []testCase{
		{
			name: "test empty query",
			postData: &model.DashboardVars{
				Query: "",
			},
			expectedErr: true,
			errMsg:      "query is required",
		},
		{
			name: "test query with no variables",
			postData: &model.DashboardVars{
				Query: "select * from table",
			},
			expectedErr: false,
			query:       "select * from table",
		},
		{
			name: "test query with variables",
			postData: &model.DashboardVars{
				Query: "select * from table where id = {{.id}}",
				Variables: map[string]interface{}{
					"id": "1",
				},
			},
			query:       "select * from table where id = '1'",
			expectedErr: false,
		},
		// While this does seem like it should throw error, it shouldn't because empty value is a valid value
		// for certain scenarios and should be treated as such.
		{
			name: "test query with variables and no value",
			postData: &model.DashboardVars{
				Query: "select * from table where id = {{.id}}",
				Variables: map[string]interface{}{
					"id": "",
				},
			},
			query:       "select * from table where id = ''",
			expectedErr: false,
		},
		{
			name: "query contains alter table",
			postData: &model.DashboardVars{
				Query: "ALTER TABLE o11y_table DELETE where true",
			},
			expectedErr: true,
			errMsg:      "operation alter table is not allowed",
		},
		{
			name: "query text produces template exec error",
			postData: &model.DashboardVars{
				Query: "SELECT durationNano from o11y_traces.o11y_index_v2 WHERE id = {{if .X}}1{{else}}2{{else}}3{{end}}",
			},
			expectedErr: true,
			errMsg:      "expected end; found {{else}}",
		},
		{
			name: "variables contain array",
			postData: &model.DashboardVars{
				Query: "SELECT operation FROM table WHERE serviceName IN {{.serviceName}}",
				Variables: map[string]interface{}{
					"serviceName": []string{"frontend", "route"},
				},
			},
			query: "SELECT operation FROM table WHERE serviceName IN ['frontend','route']",
		},
		{
			name: "query uses mixed types of variables",
			postData: &model.DashboardVars{
				Query: "SELECT body FROM table Where id = {{.id}} AND elapsed_time >= {{.elapsed_time}} AND serviceName IN {{.serviceName}}",
				Variables: map[string]interface{}{
					"serviceName":  []string{"frontend", "route"},
					"id":           "id",
					"elapsed_time": 1.24,
				},
			},
			query: "SELECT body FROM table Where id = 'id' AND elapsed_time >= 1.240000 AND serviceName IN ['frontend','route']",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			err := json.NewEncoder(&b).Encode(tc.postData)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v2/variables/query", &b)
			query, err := prepareQuery(req)

			if tc.expectedErr {
				if err == nil {
					t.Errorf("expected error, but got nil")
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("expected error message contain: %s, but got: %s", tc.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, but got: %s", err.Error())
				}
				if query != tc.query {
					t.Errorf("expected query: %s, but got: %s", tc.query, query)
				}
			}
		})
	}
}
