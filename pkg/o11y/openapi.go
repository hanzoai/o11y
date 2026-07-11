package o11y

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/apiserver"
	"github.com/hanzoai/o11y/pkg/apiserver/o11yapiserver"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/instrumentation"
	"github.com/hanzoai/o11y/pkg/modules/authdomain"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/fields"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/preference"
	"github.com/hanzoai/o11y/pkg/modules/promote"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/session"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"
	"gopkg.in/yaml.v2"
)

const o11yDiscriminatorKey string = "x-o11y-discriminator"

type OpenAPI struct {
	apiserver apiserver.APIServer
	reflector *openapi3.Reflector
	collector *handler.OpenAPICollector
}

func NewOpenAPI(ctx context.Context, instrumentation instrumentation.Instrumentation) (*OpenAPI, error) {
	apiserver, err := o11yapiserver.NewFactory(
		struct{ organization.Getter }{},
		struct{ authz.AuthZ }{},
		struct{ organization.Handler }{},
		struct{ user.Handler }{},
		struct{ session.Handler }{},
		struct{ authdomain.Handler }{},
		struct{ preference.Handler }{},
		struct{ global.Handler }{},
		struct{ promote.Handler }{},
		struct{ flagger.Handler }{},
		struct{ dashboard.Module }{},
		struct{ dashboard.Handler }{},
		struct{ metricsexplorer.Handler }{},
		struct{ metricreductionrule.Handler }{},
		struct{ inframonitoring.Handler }{},
		struct{ gateway.Handler }{},
		struct{ fields.Handler }{},
		struct{ authz.Handler }{},
		struct{ rawdataexport.Handler }{},
		struct{ zeus.Handler }{},
		struct{ querier.Handler }{},
		struct{ serviceaccount.Handler }{},
		struct{ factory.Handler }{},
		struct{ cloudintegration.Handler }{},
		struct{ rulestatehistory.Handler }{},
		struct{ spanmapper.Handler }{},
		struct{ alertmanager.Handler }{},
		struct{ llmpricingrule.Handler }{},
		struct{ tracedetail.Handler }{},
		struct{ ruler.Handler }{},
		struct{ statsreporter.Handler }{},
		struct{ llmobs.Handler }{},
		struct{ errortracking.Handler }{},
		struct{ sentry.Handler }{},
	).New(ctx, instrumentation.ToProviderSettings(), apiserver.Config{})
	if err != nil {
		return nil, err
	}

	reflector := openapi3.NewReflector()
	reflector.JSONSchemaReflector().DefaultOptions = append(reflector.JSONSchemaReflector().DefaultOptions, jsonschema.InterceptDefName(func(t reflect.Type, defaultDefName string) string {
		if defaultDefName == "RenderSuccessResponse" {
			field, ok := t.FieldByName("Data")
			if !ok {
				return defaultDefName
			}

			return field.Type.Name()
		}

		return defaultDefName
	}))

	reflector.Spec.WithInfo(*(&openapi3.Info{}).
		WithTitle("O11y").
		WithDescription("OpenTelemetry-Native Logs, Metrics and Traces in a single pane").
		WithTermsOfService("https://o11y.io/terms-of-service/").
		WithContact(*(&openapi3.Contact{}).
			WithName("O11y Support").
			WithURL("https://o11y.io").
			WithEmail("support@o11y.io")),
	)

	reflector.Spec.WithServers(
		// Default server
		*(&openapi3.Server{}).WithURL("https://{host}:{port}{base_path}").
			WithDescription("The fully qualified URL to the O11y APIServer.").
			WithVariablesItem("host", *(&openapi3.ServerVariable{}).
				WithDefault("localhost").
				WithDescription("The host of the O11y APIServer")).
			WithVariablesItem("port", *(&openapi3.ServerVariable{}).
				WithDefault("8080").
				WithDescription("The port of the O11y APIServer")).
			WithVariablesItem("base_path", *(&openapi3.ServerVariable{}).
				WithDefault("/").
				WithDescription("The base path of the O11y APIServer")),
	)

	reflector.SpecSchema().SetAPIKeySecurity(authtypes.IdentNProviderAPIKey.StringValue(), "O11y-Api-Key", openapi.InHeader, "API Keys")
	reflector.SpecSchema().SetHTTPBearerTokenSecurity(authtypes.IdentNProviderTokenizer.StringValue(), "Tokenizer", "Tokens generated by the tokenizer")

	collector := handler.NewOpenAPICollector(reflector)

	return &OpenAPI{
		apiserver: apiserver,
		reflector: reflector,
		collector: collector,
	}, nil
}

func (openapi *OpenAPI) CreateAndWrite(path string) error {
	if err := openapi.apiserver.Router().Walk(openapi.collector.Walker); err != nil {
		return err
	}

	attachDiscriminators(openapi.reflector.Spec)

	// The library's MarshalYAML does a JSON round-trip that converts all numbers
	// to float64, causing large integers (e.g. epoch millisecond timestamps) to
	// render in scientific notation (1.6409952e+12).
	jsonData, err := openapi.reflector.Spec.MarshalJSON()
	if err != nil {
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}

	convertJSONNumbers(v)

	spec, err := yaml.Marshal(v)
	if err != nil {
		return err
	}

	return os.WriteFile(path, spec, 0o600)
}

// convertJSONNumbers recursively walks a decoded JSON structure and converts
// json.Number values to int64 (preferred) or float64 so that YAML marshaling
// renders them as plain numbers instead of quoted strings.
func convertJSONNumbers(v interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, elem := range val {
			if n, ok := elem.(json.Number); ok {
				if i, err := n.Int64(); err == nil {
					val[k] = i
				} else if f, err := n.Float64(); err == nil {
					val[k] = f
				}
			} else {
				convertJSONNumbers(elem)
			}
		}
	case []interface{}:
		for i, elem := range val {
			if n, ok := elem.(json.Number); ok {
				if i64, err := n.Int64(); err == nil {
					val[i] = i64
				} else if f, err := n.Float64(); err == nil {
					val[i] = f
				}
			} else {
				convertJSONNumbers(elem)
			}
		}
	}
}

// attachDiscriminators promotes x-o11y-discriminator extensions
// into openapi3 Discriminator fields. Malformed markers are dropped.
func attachDiscriminators(spec *openapi3.Spec) {
	if spec.Components == nil || spec.Components.Schemas == nil {
		return
	}

	for name, entry := range spec.Components.Schemas.MapOfSchemaOrRefValues {
		if entry.Schema == nil {
			continue
		}

		raw, ok := entry.Schema.MapOfAnything[o11yDiscriminatorKey]
		if !ok {
			continue
		}

		marker, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		propertyName, ok := marker["propertyName"].(string)
		if !ok || propertyName == "" {
			continue
		}

		disc := openapi3.Discriminator{PropertyName: propertyName}
		if rawMapping, ok := marker["mapping"]; ok {
			if mapping, ok := rawMapping.(map[string]string); ok {
				disc.Mapping = mapping
			} else if mapping, ok := rawMapping.(map[string]any); ok {
				converted := make(map[string]string, len(mapping))
				for k, v := range mapping {
					if s, ok := v.(string); ok {
						converted[k] = s
					}
				}
				disc.Mapping = converted
			}
		}

		entry.Schema.Discriminator = &disc
		delete(entry.Schema.MapOfAnything, o11yDiscriminatorKey)

		// The parent's reflected `properties` / `required` duplicate
		// what the oneOf variants already declare, and orval intersects
		// the two — turning a clean discriminated union DTO into a
		// noisy union of intersections. Drop them here.
		entry.Schema.Properties = nil
		entry.Schema.Required = nil

		spec.Components.Schemas.MapOfSchemaOrRefValues[name] = entry
	}
}
