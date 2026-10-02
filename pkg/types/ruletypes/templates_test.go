package ruletypes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateExpander(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service.name": "my-service"}, "100", "200")
	expander := NewTemplateExpander(context.Background(), defs+"test $service.name", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test my-service", result)
}

func TestTemplateExpander_WithThreshold(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service.name": "my-service"}, "200", "100")
	expander := NewTemplateExpander(context.Background(), defs+"test $service.name exceeds {{$threshold}} and observed at {{$value}}", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test my-service exceeds 100 and observed at 200", result)
}

func TestTemplateExpanderOldVariableSyntax(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service.name": "my-service"}, "200", "100")
	expander := NewTemplateExpander(context.Background(), defs+"test {{.Labels.service_name}} exceeds {{$threshold}} and observed at {{$value}}", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test my-service exceeds 100 and observed at 200", result)
}

func TestTemplateExpander_WithAlreadyNormalizedKey(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service_name": "my-service"}, "200", "100")
	expander := NewTemplateExpander(context.Background(), defs+"test {{.Labels.service_name}} exceeds {{$threshold}} and observed at {{$value}}", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test my-service exceeds 100 and observed at 200", result)
}

func TestTemplateExpander_WithMissingKey(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service_name": "my-service"}, "200", "100")
	expander := NewTemplateExpander(context.Background(), defs+"test {{.Labels.missing_key}} exceeds {{$threshold}} and observed at {{$value}}", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test  exceeds 100 and observed at 200", result)
}

func TestTemplateExpander_WithLablesDotSyntax(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service.name": "my-service"}, "200", "100")
	expander := NewTemplateExpander(context.Background(), defs+"test {{.Labels.service.name}} exceeds {{$threshold}} and observed at {{$value}}", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test my-service exceeds 100 and observed at 200", result)
}

func TestTemplateExpander_WithVariableSyntax(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service.name": "my-service"}, "200", "100")
	expander := NewTemplateExpander(context.Background(), defs+"test {{$service.name}} exceeds {{$threshold}} and observed at {{$value}}", "test", data, nil)
	result, err := expander.Expand()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "test my-service exceeds 100 and observed at 200", result)
}

// The Prometheus spelling, {{$labels.service}}, names the label `service`. The
// {{$variable}} rewrite read it as a label called "labels.service", which no
// series carries, so every rule written that way rendered the label empty:
// "  has failed its health probe" with the service missing from the page.
func TestTemplateExpander_WithPrometheusLabelsSyntax(t *testing.T) {
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	data := AlertTemplateData(map[string]string{"service": "billing", "service.name": "my-service"}, "0", "0")
	for text, want := range map[string]string{
		"{{$labels.service}} has failed":       "billing has failed",
		"{{ $labels.service }} has failed":     "billing has failed",
		"{{$labels.service.name}} is down":     "my-service is down",
		"$labels.service has failed":           "billing has failed",
		"{{$service}} and {{$labels.service}}": "billing and billing",
	} {
		expander := NewTemplateExpander(context.Background(), defs+text, "test", data, nil)
		result, err := expander.Expand()
		require.NoError(t, err, text)
		require.Equal(t, want, result, text)
	}
}
