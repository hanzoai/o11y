import { Dispatch, SetStateAction } from 'react';
import { UseQueryResult } from 'react-query';
import { MetricsexplorertypesMetricMetadataDTO } from 'api/generated/services/o11y.schemas';
import { SuccessResponse, Warning } from 'types/api';
import { MetricRangePayloadProps } from 'types/api/metrics/getQueryRange';

export enum ExplorerTabs {
	TIME_SERIES = 'time-series',
	RELATED_METRICS = 'related-metrics',
}

export interface TimeSeriesProps {
	onFetchingStateChange?: (isFetching: boolean) => void;
	showOneChartPerQuery: boolean;
	setWarning: Dispatch<SetStateAction<Warning | undefined>>;
	areAllMetricUnitsSame: boolean;
	isMetricUnitsLoading: boolean;
	isMetricUnitsError: boolean;
	metricUnits: (string | undefined)[];
	metricNames: string[];
	metrics: (MetricsexplorertypesMetricMetadataDTO | undefined)[];
	handleOpenMetricDetails: (metricName: string) => void;
	yAxisUnit: string | undefined;
	setYAxisUnit: (unit: string) => void;
	showYAxisUnitSelector: boolean;
	isCancelled?: boolean;
}
