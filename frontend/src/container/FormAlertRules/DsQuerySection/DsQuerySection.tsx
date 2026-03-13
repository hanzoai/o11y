import DatastoreQueryBuilder from 'container/NewWidget/LeftContainer/QuerySection/QueryBuilder/datastore/query';
import { useQueryBuilder } from 'hooks/queryBuilder/useQueryBuilder';

function DsQuerySection(): JSX.Element {
	const { currentQuery } = useQueryBuilder();

	return (
		<DatastoreQueryBuilder
			key="A"
			queryIndex={0}
			queryData={currentQuery.datastore_sql[0]}
			deletable={false}
		/>
	);
}

export default DsQuerySection;
