import { Color } from 'constants/designTokens';
import { Button, Typography } from 'antd';
import { Info } from 'lucide-react';

import { MetricDetailsErrorStateProps } from './types';

import type { JSX } from 'react';

function MetricDetailsErrorState({
	refetch,
	errorMessage,
}: MetricDetailsErrorStateProps): JSX.Element {
	return (
		<div className="metric-details-error-state">
			<Info size={20} color={Color.BG_CHERRY_500} />
			<Typography.Text>{errorMessage}</Typography.Text>
			{refetch && <Button onClick={refetch}>Retry</Button>}
		</div>
	);
}

export default MetricDetailsErrorState;
