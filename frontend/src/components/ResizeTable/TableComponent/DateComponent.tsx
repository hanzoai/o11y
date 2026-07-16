import { Typography } from 'components/ui/typography';

import Time from './Time';

import type { JSX } from 'react';

function DateComponent(
	CreatedOrUpdateTime: string | number | Date,
): JSX.Element {
	if (CreatedOrUpdateTime === null) {
		return <Typography> - </Typography>;
	}

	return <Time CreatedOrUpdateTime={CreatedOrUpdateTime} />;
}

export default DateComponent;
