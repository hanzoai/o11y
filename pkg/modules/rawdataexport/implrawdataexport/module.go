package implrawdataexport

import (
	"context"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/types/ctxtypes"
	"github.com/hanzoai/o11y/pkg/types/instrumentationtypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Module struct {
	querier querier.Querier
}

func NewModule(querier querier.Querier) rawdataexport.Module {
	return &Module{
		querier: querier,
	}
}

func (m *Module) ExportRawData(ctx context.Context, orgID valuer.UUID, rangeRequest *qbtypes.QueryRangeRequest, doneChan chan any) (chan *qbtypes.RawRow, chan error) {
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.CodeNamespace:    "rawdataexport",
		instrumentationtypes.CodeFunctionName: "ExportRawData",
	})

	spec := rangeRequest.CompositeQuery.Queries[0].Spec.(qbtypes.QueryBuilderQuery[qbtypes.LogAggregation])
	rowCountLimit := spec.Limit

	rowChan := make(chan *qbtypes.RawRow, 1)
	errChan := make(chan error, 1)

	go func() {
		// Set clickhouse max threads
		ctx := ctxtypes.SetClickhouseMaxThreads(ctx, ClickhouseExportRawDataMaxThreads)
		// Set clickhouse timeout
		contextWithTimeout, cancel := context.WithTimeout(ctx, ClickhouseExportRawDataTimeout)
		defer cancel()
		defer close(errChan)
		defer close(rowChan)

		rowCount := 0

		for rowCount < rowCountLimit {
			spec.Limit = min(ChunkSize, rowCountLimit-rowCount)
			spec.Offset = rowCount

			rangeRequest.CompositeQuery.Queries[0].Spec = spec

			response, err := m.querier.QueryRange(contextWithTimeout, orgID, rangeRequest)
			if err != nil {
				errChan <- err
				return
			}

			newRowsCount := 0
			for _, result := range response.Data.Results {
				resultData, ok := result.(*qbtypes.RawData)
				if !ok {
					errChan <- errors.NewInternalf(errors.CodeInternal, "expected RawData, got %T", result)
					return
				}

				newRowsCount += len(resultData.Rows)
				for _, row := range resultData.Rows {
					select {
					case rowChan <- row:
					case <-doneChan:
						return
					case <-ctx.Done():
						errChan <- ctx.Err()
						return
					}
				}

			}

			// Break if we did not receive any new rows
			if newRowsCount == 0 {
				return
			}

			rowCount += newRowsCount

		}
	}()

	return rowChan, errChan

}
