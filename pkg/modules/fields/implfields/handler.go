package implfields

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/fields"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

type handler struct {
	telemetryMetadataStore telemetrytypes.MetadataStore
}

func NewHandler(settings factory.ProviderSettings, telemetryMetadataStore telemetrytypes.MetadataStore) fields.Handler {
	return &handler{
		telemetryMetadataStore: telemetryMetadataStore,
	}
}

func (handler *handler) GetFieldsKeys(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var params telemetrytypes.PostableFieldKeysParams
	if err := binding.Query.BindQuery(req.URL.Query(), &params); err != nil {
		render.Error(rw, err)
		return
	}

	fieldKeySelector := telemetrytypes.NewFieldKeySelectorFromPostableFieldKeysParams(params)

	keys, complete, err := handler.telemetryMetadataStore.GetKeys(ctx, fieldKeySelector)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, &telemetrytypes.GettableFieldKeys{
		Keys:     keys,
		Complete: complete,
	})
}

func (handler *handler) GetFieldsValues(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var params telemetrytypes.PostableFieldValueParams
	if err := binding.Query.BindQuery(req.URL.Query(), &params); err != nil {
		render.Error(rw, err)
		return
	}

	fieldValueSelector := telemetrytypes.NewFieldValueSelectorFromPostableFieldValueParams(params)

	allValues, allComplete, err := handler.telemetryMetadataStore.GetAllValues(ctx, fieldValueSelector)
	if err != nil {
		render.Error(rw, err)
		return
	}

	relatedValues, relatedComplete, err := handler.telemetryMetadataStore.GetRelatedValues(ctx, fieldValueSelector)
	if err != nil {
		// we don't want to return error if we fail to get related values for some reason
		relatedValues = []string{}
	}

	values := &telemetrytypes.TelemetryFieldValues{
		StringValues:  allValues.StringValues,
		NumberValues:  allValues.NumberValues,
		RelatedValues: relatedValues,
	}

	render.Success(rw, http.StatusOK, &telemetrytypes.GettableFieldValues{
		Values:   values,
		Complete: allComplete && relatedComplete,
	})
}
