import { useTranslation } from 'react-i18next';
import Spinner from 'components/Spinner';

import type { JSX } from 'react';

interface DataStateRendererProps<T> {
	isLoading: boolean;
	isRefetching: boolean;
	isError: boolean;
	data: T | null;
	errorMessage?: string;
	loadingMessage?: string;
	children: (data: T) => React.ReactNode;
}

/**
 *  TODO(shaheer): add empty state and optionally accept empty state custom component
 *  TODO(shaheer): optionally accept custom error state component
 *  TODO(shaheer): optionally accept custom loading state component
 */
function DataStateRenderer<T>({
	isLoading,
	isRefetching,
	isError,
	data,
	errorMessage = '',
	loadingMessage = 'Loading...',
	children,
}: DataStateRendererProps<T>): JSX.Element {
	const { t } = useTranslation('common');

	if (isLoading || isRefetching || !data) {
		return <Spinner tip={loadingMessage} height="100%" />;
	}

	if (isError || data === null) {
		return <div>{errorMessage ?? t('something_went_wrong')}</div>;
	}

	return <>{children(data)}</>;
}

export default DataStateRenderer;
