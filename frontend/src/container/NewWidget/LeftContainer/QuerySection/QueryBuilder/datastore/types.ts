import { IDatastoreQuery } from 'types/api/queryBuilder/queryBuilderData';

export interface IDatastoreQueryHandleChange {
	queryIndex: number | string;
	query?: IDatastoreQuery['query'];
	legend?: IDatastoreQuery['legend'];
	toggleDisable?: IDatastoreQuery['disabled'];
	toggleDelete?: boolean;
}
