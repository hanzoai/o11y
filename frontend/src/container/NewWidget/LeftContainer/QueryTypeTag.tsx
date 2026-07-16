import { EQueryType } from 'types/common/dashboard';

import type { JSX } from 'react';

function QueryTypeTag({
	queryType = EQueryType.QUERY_BUILDER,
}: IQueryTypeTagProps): JSX.Element {
	switch (queryType) {
		case EQueryType.QUERY_BUILDER:
			return <span>Query Builder</span>;

		case EQueryType.DATASTORE:
			return <span>Datastore Query</span>;
		case EQueryType.PROM:
			return <span>PromQL</span>;
		default:
			return <span />;
	}
}

interface IQueryTypeTagProps {
	queryType?: EQueryType;
}

export default QueryTypeTag;
