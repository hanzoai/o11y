import { useMemo, useState, type JSX } from 'react';
import { QueryKey } from 'react-query';
// eslint-disable-next-line no-restricted-imports
import { useSelector } from 'react-redux';
import localStorageGet from 'api/browser/localstorage/get';
import localStorageSet from 'api/browser/localstorage/set';
import ErrorInPlace from 'components/ErrorInPlace/ErrorInPlace';
import Spinner from 'components/Spinner';
import { SKIP_ONBOARDING } from 'constants/onboarding';
import useGetTopLevelOperations from 'hooks/useGetTopLevelOperations';
import useResourceAttribute from 'hooks/useResourceAttribute';
import { convertRawQueriesToTraceSelectedTags } from 'hooks/useResourceAttribute/utils';
import { AppState } from 'store/reducers';
import APIError from 'types/api/error';
import { GlobalReducer } from 'types/reducer/globalTime';
import { Tags } from 'types/reducer/trace';

import SkipOnBoardingModal from '../SkipOnBoardModal';
import ServiceMetricsApplication from './ServiceMetricsApplication';

function ServicesUsingMetrics(): JSX.Element {
	const {
		maxTime,
		minTime,
		selectedTime: globalSelectedInterval,
	} = useSelector<AppState, GlobalReducer>((state) => state.globalTime);
	const { queries } = useResourceAttribute();
	const selectedTags = useMemo(
		() => (convertRawQueriesToTraceSelectedTags(queries) as Tags[]) || [],
		[queries],
	);

	const queryKey: QueryKey = [
		minTime,
		maxTime,
		selectedTags,
		globalSelectedInterval,
	];
	const { data, error, isLoading, isError } = useGetTopLevelOperations(queryKey, {
		start: minTime,
		end: maxTime,
	});

	const [skipOnboarding, setSkipOnboarding] = useState(
		localStorageGet(SKIP_ONBOARDING) === 'true',
	);

	const onContinueClick = (): void => {
		localStorageSet(SKIP_ONBOARDING, 'true');
		setSkipOnboarding(true);
	};

	const topLevelOperations = Object.entries(data || {});

	// A FAILED READ IS NOT AN EMPTY ONE — and this branch was worse than its
	// sibling in ServiceTraces: it never looked at the data at all. `isError ===
	// true` was the WHOLE condition, so the "instrument your application" modal
	// was shown if and only if the request FAILED, and a genuinely empty
	// deployment — the one case onboarding exists for — never saw it.
	//
	// Now: a failure shows the failure, a successful empty read offers
	// onboarding, and neither is mistaken for the other.
	if (isError) {
		return <ErrorInPlace error={error as APIError} />;
	}

	if (isLoading) {
		return <Spinner tip="Loading..." />;
	}

	if (topLevelOperations.length === 0 && !skipOnboarding) {
		return <SkipOnBoardingModal onContinueClick={onContinueClick} />;
	}

	return <ServiceMetricsApplication topLevelOperations={topLevelOperations} />;
}

export default ServicesUsingMetrics;
