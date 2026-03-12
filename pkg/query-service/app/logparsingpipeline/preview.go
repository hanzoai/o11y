package logparsingpipeline

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/hanzoai/otel-collector/pkg/collectorsimulator"
	_ "github.com/hanzoai/otel-collector/pkg/parser/grok"
	"github.com/hanzoai/otel-collector/processor/o11ylogspipelineprocessor"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/hanzoai/o11y/pkg/types/pipelinetypes"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func SimulatePipelinesProcessing(ctx context.Context, pipelines []pipelinetypes.GettablePipeline, logs []model.O11yLog) (
	[]model.O11yLog, []string, error) {
	if len(pipelines) < 1 {
		return logs, nil, nil
	}

	// Collector simulation does not guarantee that logs will come
	// out in the same order as in the input.
	//
	// Add a temp attribute for sorting logs in simulation output
	inputOrderAttribute := "__o11y_input_idx__"
	for i := 0; i < len(logs); i++ {
		if logs[i].Attributes_int64 == nil {
			logs[i].Attributes_int64 = map[string]int64{}
		}
		logs[i].Attributes_int64[inputOrderAttribute] = int64(i)
	}
	simulatorInputPLogs := O11yLogsToPLogs(logs)

	processorFactories, err := otelcol.MakeFactoryMap(o11ylogspipelineprocessor.NewFactory())
	if err != nil {
		return nil, nil, errors.WrapInternalf(err, CodeProcessorFactoryMapFailed, "could not construct processor factory map")
	}

	// Pipelines translate to logtransformprocessors in otel collector config.
	// Each logtransformprocessor (stanza) does its own batching with a flush
	// interval of 100ms. So e2e processing time for logs grows linearly with
	// the number of logtransformprocessors involved.
	// See defaultFlushInterval at https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/stanza/adapter/emitter.go
	// TODO(Raj): Remove this after flushInterval is exposed in logtransformprocessor config
	timeout := time.Millisecond * time.Duration(len(pipelines)*100+100)

	configGenerator := func(baseConf []byte) ([]byte, error) {
		updatedConf, err := GenerateCollectorConfigWithPipelines(baseConf, pipelines)
		if err != nil {
			return nil, err
		}
		return updatedConf, nil
	}

	outputPLogs, collectorErrs, simulationErr := collectorsimulator.SimulateLogsProcessing(
		ctx,
		processorFactories,
		configGenerator,
		simulatorInputPLogs,
		timeout,
	)
	if simulationErr != nil {
		if errors.Is(simulationErr, collectorsimulator.ErrInvalidConfig) {
			return nil, nil, errors.WrapInvalidInputf(simulationErr, errors.CodeInvalidInput, "invalid config")
		}
		return nil, nil, errors.WrapInternalf(simulationErr, errors.CodeInternal, "could not simulate log pipelines processing")
	}

	outputO11yLogs := PLogsToO11yLogs(outputPLogs)

	// Sort output logs by their order in the input and remove the temp ordering attribute
	sort.Slice(outputO11yLogs, func(i, j int) bool {
		iIdx := outputO11yLogs[i].Attributes_int64[inputOrderAttribute]
		jIdx := outputO11yLogs[j].Attributes_int64[inputOrderAttribute]
		return iIdx < jIdx
	})
	for _, sigLog := range outputO11yLogs {
		delete(sigLog.Attributes_int64, inputOrderAttribute)
	}

	collectorWarnAndErrorLogs := []string{}
	for _, log := range collectorErrs {
		// if log is empty or log comes from featuregate.go, then remove it
		if log == "" || strings.Contains(log, "featuregate.go") {
			continue
		}
		collectorWarnAndErrorLogs = append(collectorWarnAndErrorLogs, log)
	}

	return outputO11yLogs, collectorWarnAndErrorLogs, nil
}

// plog doesn't contain an ID field.
// O11yLog.ID is stored as a log attribute in plogs for processing
// and gets hydrated back later.
const O11yLogIdAttr = "__o11y_log_id__"

func O11yLogsToPLogs(logs []model.O11yLog) []plog.Logs {
	result := []plog.Logs{}

	for _, log := range logs {
		pl := plog.NewLogs()
		rl := pl.ResourceLogs().AppendEmpty()

		resourceAttribs := rl.Resource().Attributes()
		for k, v := range log.Resources_string {
			resourceAttribs.PutStr(k, v)
		}

		scopeLog := rl.ScopeLogs().AppendEmpty()
		slRecord := scopeLog.LogRecords().AppendEmpty()

		slRecord.SetTimestamp(pcommon.NewTimestampFromTime(
			time.Unix(0, int64(log.Timestamp)),
		))

		var traceIdBuf [16]byte
		copy(traceIdBuf[:], []byte(log.TraceID))
		slRecord.SetTraceID(traceIdBuf)

		var spanIdBuf [8]byte
		copy(spanIdBuf[:], []byte(log.SpanID))
		slRecord.SetSpanID(spanIdBuf)

		slRecord.SetFlags(plog.LogRecordFlags(log.TraceFlags))

		slRecord.SetSeverityText(log.SeverityText)
		slRecord.SetSeverityNumber(plog.SeverityNumber(log.SeverityNumber))

		slRecord.Body().FromRaw(log.Body)

		slAttribs := slRecord.Attributes()
		for k, v := range log.Attributes_int64 {
			slAttribs.PutInt(k, v)
		}
		for k, v := range log.Attributes_float64 {
			slAttribs.PutDouble(k, v)
		}
		for k, v := range log.Attributes_string {
			slAttribs.PutStr(k, v)
		}
		slAttribs.PutStr(O11yLogIdAttr, log.ID)

		result = append(result, pl)
	}

	return result
}

func PLogsToO11yLogs(plogs []plog.Logs) []model.O11yLog {
	result := []model.O11yLog{}

	for _, pl := range plogs {

		resourceLogsSlice := pl.ResourceLogs()
		for i := 0; i < resourceLogsSlice.Len(); i++ {
			rl := resourceLogsSlice.At(i)

			scopeLogsSlice := rl.ScopeLogs()
			for j := 0; j < scopeLogsSlice.Len(); j++ {
				sl := scopeLogsSlice.At(j)

				lrSlice := sl.LogRecords()
				for k := 0; k < lrSlice.Len(); k++ {
					lr := lrSlice.At(k)

					// Recover ID for the log and remove temp attrib used for storing it
					o11yLogId := ""
					logIdVal, exists := lr.Attributes().Get(O11yLogIdAttr)
					if exists {
						o11yLogId = logIdVal.Str()
					}
					lr.Attributes().Remove(O11yLogIdAttr)

					o11yLog := model.O11yLog{
						Timestamp:          uint64(lr.Timestamp()),
						ID:                 o11yLogId,
						TraceID:            lr.TraceID().String(),
						SpanID:             lr.SpanID().String(),
						TraceFlags:         uint32(lr.Flags()),
						SeverityText:       lr.SeverityText(),
						SeverityNumber:     uint8(lr.SeverityNumber()),
						Body:               lr.Body().AsString(),
						Resources_string:   pMapToStrMap(rl.Resource().Attributes()),
						Attributes_string:  map[string]string{},
						Attributes_int64:   map[string]int64{},
						Attributes_float64: map[string]float64{},
					}

					// Populate o11yLog.Attributes_...
					lr.Attributes().Range(func(k string, v pcommon.Value) bool {
						if v.Type() == pcommon.ValueTypeDouble {
							o11yLog.Attributes_float64[k] = v.Double()
						} else if v.Type() == pcommon.ValueTypeInt {
							o11yLog.Attributes_int64[k] = v.Int()
						} else {
							o11yLog.Attributes_string[k] = v.AsString()
						}
						return true
					})

					result = append(result, o11yLog)
				}
			}
		}
	}

	return result
}

func pMapToStrMap(pMap pcommon.Map) map[string]string {
	result := map[string]string{}
	pMap.Range(func(k string, v pcommon.Value) bool {
		result[k] = v.AsString()
		return true
	})
	return result
}
