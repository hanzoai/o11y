import { useEffect, useMemo, useRef, useState, type JSX } from 'react';
// eslint-disable-next-line no-restricted-imports
import { useSelector } from 'react-redux';
import localStorageGet from 'api/browser/localstorage/get';
import localStorageSet from 'api/browser/localstorage/set';
import logEvent from 'api/common/logEvent';
import ErrorInPlace from 'components/ErrorInPlace/ErrorInPlace';
import { SKIP_ONBOARDING } from 'constants/onboarding';
import useErrorNotification from 'hooks/useErrorNotification';
import { useQueryService } from 'hooks/useQueryService';
import useResourceAttribute from 'hooks/useResourceAttribute';
import {
	convertRawQueriesToTraceSelectedTags,
	getResourceDeploymentKeys,
} from 'hooks/useResourceAttribute/utils';
import { isUndefined } from 'lodash-es';
import { AppState } from 'store/reducers';
import APIError from 'types/api/error';
import { GlobalReducer } from 'types/reducer/globalTime';
import { Tags } from 'types/reducer/trace';

import { FeatureKeys } from '../../../constants/features';
import { useAppContext } from '../../../providers/App/App';
import SkipOnBoardingModal from '../SkipOnBoardModal';
import ServiceTraceTable from './ServiceTracesTable';

function ServiceTraces(): JSX.Element {
	const { maxTime, minTime, selectedTime } = useSelector<
		AppState,
		GlobalReducer
	>((state) => state.globalTime);
	const { queries } = useResourceAttribute();
	const selectedTags = useMemo(
		() => (convertRawQueriesToTraceSelectedTags(queries) as Tags[]) || [],
		[queries],
	);

	const { data, error, isLoading, isError } = useQueryService({
		minTime,
		maxTime,
		selectedTime,
		selectedTags,
	});

	const { featureFlags } = useAppContext();
	const dotMetricsEnabled =
		featureFlags?.find((flag) => flag.name === FeatureKeys.DOT_METRICS_ENABLED)
			?.active || false;

	useErrorNotification(error);

	const services = data || [];

	const [skipOnboarding, setSkipOnboarding] = useState(
		localStorageGet(SKIP_ONBOARDING) === 'true',
	);

	const onContinueClick = (): void => {
		localStorageSet(SKIP_ONBOARDING, 'true');
		setSkipOnboarding(true);
	};

	const logEventCalledRef = useRef(false);
	useEffect(() => {
		if (!logEventCalledRef.current && !isUndefined(data)) {
			const selectedEnvironments = queries.find(
				(val) => val.tagKey === getResourceDeploymentKeys(dotMetricsEnabled),
			)?.tagValue;

			const rps = data.reduce((total, service) => total + service.callRate, 0);

			logEvent('APM: List page visited', {
				numberOfServices: data?.length,
				selectedEnvironments,
				resourceAttributeUsed: !!queries?.length,
				rps,
			});
			logEventCalledRef.current = true;
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [data]);

	// A FAILED READ IS NOT AN EMPTY ONE.
	//
	// This branch used to require `isError === true` and then render the
	// "instrument your application" onboarding modal — so the ONLY way to reach
	// that modal was for the request to FAIL. When HIP-0132 dropped the database
	// this list reads, every customer with a fully instrumented fleet was told
	// their fleet was not instrumented, and the actual reason
	// (`Code: 81 UNKNOWN_DATABASE`) appeared nowhere on the page.
	//
	// The two states are separate now and each says what is true: a failed read
	// shows the failure, and only a SUCCESSFUL read that returned nothing offers
	// onboarding.
	if (isError) {
		return <ErrorInPlace error={error as unknown as APIError} />;
	}

	if (services.length === 0 && isLoading === false && !skipOnboarding) {
		return <SkipOnBoardingModal onContinueClick={onContinueClick} />;
	}

	return <ServiceTraceTable services={services} loading={isLoading} />;
}

export default ServiceTraces;
