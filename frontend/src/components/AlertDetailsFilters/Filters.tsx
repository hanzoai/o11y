import DateTimeSelector from 'container/TopNav/DateTimeSelectionV2';

import './Filters.styles.scss';

import type { JSX } from 'react';

export function Filters(): JSX.Element {
	return (
		<div className="filters">
			<DateTimeSelector showAutoRefresh={false} hideShareModal showResetButton />
		</div>
	);
}
