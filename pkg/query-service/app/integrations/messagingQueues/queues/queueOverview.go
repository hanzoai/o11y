package queues

import (
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
)

func BuildOverviewQuery(queueList *QueueListRequest) (*v3.DatastoreQuery, error) {

	err := queueList.Validate()
	if err != nil {
		return nil, err
	}

	query := generateOverviewSQL(queueList.Start, queueList.End, queueList.Filters.Items)

	return &v3.DatastoreQuery{
		Query: query,
	}, nil
}
