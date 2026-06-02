import { Plus } from '@signozhq/icons';
import { Callout } from '@signozhq/ui/callout';
import { useQueryBuilder } from 'hooks/queryBuilder/useQueryBuilder';
import { EQueryType } from 'types/common/dashboard';
import DOCLINKS from 'utils/docLinks';

import { QueryButton } from '../../styles';
import DatastoreQueryBuilder from './query';

function DatastoreQueryContainer(): JSX.Element | null {
	const { currentQuery, addNewQueryItem } = useQueryBuilder();
	const addQueryHandler = (): void => {
		addNewQueryItem(EQueryType.DATASTORE);
	};

	return (
		<>
			{currentQuery.datastore_sql.map((q, idx) => (
				<DatastoreQueryBuilder
					key={q.name}
					queryIndex={idx}
					deletable={currentQuery.datastore_sql.length > 1}
					queryData={q}
				/>
			))}
			<QueryButton
				onClick={addQueryHandler}
				icon={<Plus size="md" />}
				style={{ margin: '0.4rem 1rem' }}
			>
				Query
			</QueryButton>
		</>
	);
}

export default DatastoreQueryContainer;
