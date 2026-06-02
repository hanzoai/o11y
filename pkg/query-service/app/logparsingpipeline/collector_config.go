package logparsingpipeline

import (
	"encoding/json"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/query-service/constants"
	"github.com/hanzoai/o11y/pkg/types/pipelinetypes"
	"go.uber.org/zap"
)

var lockLogsPipelineSpec sync.RWMutex

var (
	CodeCollectorConfigUnmarshalFailed        = errors.MustNewCode("collector_config_unmarshal_failed")
	CodeCollectorConfigMarshalFailed          = errors.MustNewCode("collector_config_marshal_failed")
	CodeCollectorConfigServiceNotFound        = errors.MustNewCode("collector_config_service_not_found")
	CodeCollectorConfigServiceMarshalFailed   = errors.MustNewCode("collector_config_service_marshal_failed")
	CodeCollectorConfigServiceUnmarshalFailed = errors.MustNewCode("collector_config_service_unmarshal_failed")
	CodeCollectorConfigLogsPipelineNotFound   = errors.MustNewCode("collector_config_logs_pipeline_not_found")
)

const (
	memoryLimiterProcessor       = "memory_limiter"
	memoryLimiterProcessorPrefix = "memory_limiter/"
	batchProcessor               = "batch"
	batchProcessorPrefix         = "batch/"
)

// check if the processors already exist
// if yes then update the processor.
// if something doesn't exists then remove it.
func updateProcessorConfigsInCollectorConf(
	collectorConf map[string]interface{},
	o11yPipelineProcessors map[string]interface{},
) error {
	agentProcessors := map[string]interface{}{}
	if collectorConf["processors"] != nil {
		agentProcessors = (collectorConf["processors"]).(map[string]interface{})
	}

	exists := map[string]struct{}{}
	for key, params := range o11yPipelineProcessors {
		agentProcessors[key] = params
		exists[key] = struct{}{}
	}
	// remove the old unwanted pipeline processors
	for k := range agentProcessors {
		_, isInDesiredPipelineProcs := exists[k]
		if hasO11yPipelineProcessorPrefix(k) && !isInDesiredPipelineProcs {
			delete(agentProcessors, k)
		}
	}
	collectorConf["processors"] = agentProcessors
	return nil
}

type otelPipeline struct {
	Pipelines struct {
		Logs *struct {
			Exporters  []string `json:"exporters" yaml:"exporters"`
			Processors []string `json:"processors" yaml:"processors"`
			Receivers  []string `json:"receivers" yaml:"receivers"`
		} `json:"logs" yaml:"logs"`
	} `json:"pipelines" yaml:"pipelines"`
}

func getOtelPipelineFromConfig(config map[string]interface{}) (*otelPipeline, error) {
	if _, ok := config["service"]; !ok {
		return nil, errors.NewInvalidInputf(CodeCollectorConfigServiceNotFound, "service not found in OTEL config")
	}
	b, err := json.Marshal(config["service"])
	if err != nil {
		return nil, errors.WrapInternalf(err, CodeCollectorConfigServiceMarshalFailed, "could not marshal OTEL config")
	}
	p := otelPipeline{}
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, errors.WrapInternalf(err, CodeCollectorConfigServiceUnmarshalFailed, "could not unmarshal OTEL config")
	}
	return &p, nil
}

// buildCollectorPipelineProcessorsList assembles the final processor list in the
// required order:
//
//  1. memory_limiter processors (any processor named "memory_limiter" or "memory_limiter/<id>")
//  2. signoz user-pipeline processors (in the order given by signozPipelineProcessorNames)
//  3. custom processors (non-signoz, non-memory_limiter, non-batch processors from the current config)
//  4. batch processors (any processor named "batch" or "batch/<id>") and anything after them
func buildCollectorPipelineProcessorsList(
	currentCollectorProcessors []string,
	o11yPipelineProcessorNames []string,
) ([]string, error) {
	lockLogsPipelineSpec.Lock()
	defer lockLogsPipelineSpec.Unlock()

	exists := map[string]struct{}{}
	for _, v := range o11yPipelineProcessorNames {
		exists[v] = struct{}{}
	}

	// removed the old processors which are not used
	var pipeline []string
	for _, procName := range currentCollectorProcessors {
		_, isInDesiredPipelineProcs := exists[procName]
		if isInDesiredPipelineProcs || !hasO11yPipelineProcessorPrefix(procName) {
			pipeline = append(pipeline, procName)
		}
	}

	// create a reverse map of existing config processors and their position
	existing := map[string]int{}
	for i, p := range pipeline {
		name := p
		existing[name] = i
	}

	// create mapping from our logsParserPipeline to position in existing processors (from current config)
	// this means, if "batch" holds position 3 in the current effective config, and 2 in our config, the map will be [2]: 3
	specVsExistingMap := map[int]int{}
	existingVsSpec := map[int]int{}

	// go through plan and map its elements to current positions in effective config
	for i, m := range o11yPipelineProcessorNames {
		if loc, ok := existing[m]; ok {
			specVsExistingMap[i] = loc
			existingVsSpec[loc] = i
		}
	}

	lastMatched := 0
	newPipeline := []string{}

	for i := 0; i < len(o11yPipelineProcessorNames); i++ {
		m := o11yPipelineProcessorNames[i]
		if loc, ok := specVsExistingMap[i]; ok {
			for j := lastMatched; j < loc; j++ {
				if hasO11yPipelineProcessorPrefix(pipeline[j]) {
					delete(specVsExistingMap, existingVsSpec[j])
				} else {
					newPipeline = append(newPipeline, pipeline[j])
				}
			}
		}
		if batchIdx >= 0 {
			break
		}
	}

	result := make([]string, 0, len(currentCollectorProcessors)+len(signozPipelineProcessorNames))
	result = append(result, memoryLimiters...)
	result = append(result, signozPipelineProcessorNames...)
	result = append(result, customProcessors...)
	if batchIdx >= 0 {
		result = append(result, currentCollectorProcessors[batchIdx:]...)
	}
	return result, nil
}

func GenerateCollectorConfigWithPipelines(config []byte, pipelines []pipelinetypes.GettablePipeline) ([]byte, error) {
	var collectorConf map[string]interface{}
	err := yaml.Unmarshal([]byte(config), &collectorConf)
	if err != nil {
		return nil, errors.WrapInvalidInputf(err, CodeCollectorConfigUnmarshalFailed, "could not unmarshal collector config")
	}

	o11yPipelineProcessors, o11yPipelineProcNames, err := PreparePipelineProcessor(pipelines)
	if err != nil {
		return nil, err
	}

	// Escape any `$`s as `$$$` in config generated for pipelines, to ensure any occurrences
	// like $data do not end up being treated as env vars when loading collector config.
	for _, procName := range o11yPipelineProcNames {
		procConf := o11yPipelineProcessors[procName]
		serializedProcConf, err := yaml.Marshal(procConf)
		if err != nil {
			return nil, errors.WrapInternalf(err, CodeCollectorConfigMarshalFailed, "could not marshal processor config for %s", procName)
		}
		escapedSerializedConf := strings.ReplaceAll(
			string(serializedProcConf), "$", "$$",
		)

		var escapedConf map[string]interface{}
		err = yaml.Unmarshal([]byte(escapedSerializedConf), &escapedConf)
		if err != nil {
			return nil, errors.WrapInternalf(err, CodeCollectorConfigUnmarshalFailed, "could not unmarshal dollar escaped processor config for %s", procName)
		}

		o11yPipelineProcessors[procName] = escapedConf
	}

	// Add processors to unmarshaled collector config `c`
	updateProcessorConfigsInCollectorConf(collectorConf, o11yPipelineProcessors)

	// build the new processor list in service.pipelines.logs
	p, err := getOtelPipelineFromConfig(collectorConf)
	if err != nil {
		return nil, err
	}
	if p.Pipelines.Logs == nil {
		return nil, errors.NewInternalf(CodeCollectorConfigLogsPipelineNotFound, "logs pipeline doesn't exist")
	}

	updatedProcessorList, _ := buildCollectorPipelineProcessorsList(p.Pipelines.Logs.Processors, o11yPipelineProcNames)
	p.Pipelines.Logs.Processors = updatedProcessorList

	// add the new processor to the data ( no checks required as the keys will exists)
	collectorConf["service"].(map[string]interface{})["pipelines"].(map[string]interface{})["logs"] = p.Pipelines.Logs

	updatedConf, err := yaml.Marshal(collectorConf)
	if err != nil {
		return nil, errors.WrapInternalf(err, CodeCollectorConfigMarshalFailed, "could not marshal collector config")
	}

	return updatedConf, nil
}

func hasO11yPipelineProcessorPrefix(procName string) bool {
	return strings.HasPrefix(procName, constants.LogsPPLPfx) || strings.HasPrefix(procName, constants.OldLogsPPLPfx)
}
