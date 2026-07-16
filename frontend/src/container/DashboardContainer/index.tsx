import { useFullScreenHandle } from 'react-full-screen';

import Description from './DashboardDescription';
import GridGraphs from './GridGraphs';

import type { JSX } from 'react';

function DashboardContainer(): JSX.Element {
	const handle = useFullScreenHandle();
	return (
		<div>
			<Description handle={handle} />
			<GridGraphs handle={handle} />
		</div>
	);
}

export default DashboardContainer;
