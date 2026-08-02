package handler

import (
	"net/http"
	"reflect"

	"github.com/swaggest/jsonschema-go"
	openapigo "github.com/swaggest/openapi-go"
	"github.com/swaggest/rest/openapi"
)

// OpenAPIExample is a named example for an OpenAPI operation.
type OpenAPIExample struct {
	Name        string
	Summary     string
	Description string
	Value       any
}

// Def is the definition of an OpenAPI operation.
type OpenAPIDef struct {
	ID                  string
	Tags                []string
	Summary             string
	Description         string
	Request             any
	RequestQuery        any
	RequestContentType  string
	RequestExamples     []OpenAPIExample
	Response            any
	ResponseContentType string
	SuccessStatusCode   int
	ErrorStatusCodes    []int
	Deprecated          bool
	SecuritySchemes     []OpenAPISecurityScheme
}

type OpenAPISecurityScheme struct {
	Name   string
	Scopes []string
}

// OpenAPICollector is a collector for OpenAPI operations.
type OpenAPICollector struct {
	collector *openapi.Collector
}

func NewOpenAPICollector(reflector openapigo.Reflector) *OpenAPICollector {
	c := openapi.NewCollector(reflector)

	return &OpenAPICollector{
		collector: c,
	}
}

// Collect adds one registered route to the document. A route whose handler
// declares nothing contributes nothing — a plain http.HandlerFunc has no
// operation to describe, and that is not an error.
//
// It takes the registration's three values rather than a route object to walk,
// because a walk is a router's API and the document is not the router's
// business: the caller holds the table its own registrar recorded and hands over
// exactly what an operation is made of.
func (c *OpenAPICollector) Collect(method, path string, h http.Handler) error {
	declared, ok := h.(Handler)
	if !ok || path == "" || method == "" {
		return nil
	}

	return c.collector.CollectOperation(method, path, c.collect(method, path, declared.ServeOpenAPI))
}

func (c *OpenAPICollector) collect(method string, path string, serveOpenAPIFunc ServeOpenAPIFunc) func(oc openapigo.OperationContext) error {
	return func(oc openapigo.OperationContext) error {
		// Serve the OpenAPI documentation for the handler
		serveOpenAPIFunc(oc)

		// If the handler has annotations, skip the collection
		if c.collector.HasAnnotation(method, path) {
			return nil
		}

		// Automatically sanitize the method and path
		_, _, pathItems, err := openapigo.SanitizeMethodPath(method, path)
		if err != nil {
			return err
		}

		// If there are path items, add them to the request structure
		if len(pathItems) > 0 {
			req := jsonschema.Struct{}
			for _, p := range pathItems {
				req.Fields = append(req.Fields, jsonschema.Field{
					Name:  "F" + p,
					Tag:   reflect.StructTag(`path:"` + p + `"`),
					Value: "",
				})
			}

			oc.AddReqStructure(req)
		}

		return nil
	}
}
