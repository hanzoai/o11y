import { FullScreenHandle } from 'react-full-screen';

import GraphLayoutContainer from './GridCardLayout';

import type { JSX } from 'react';

interface GridGraphProps {
	handle: FullScreenHandle;
	enableDrillDown?: boolean;
}
function GridGraph(props: GridGraphProps): JSX.Element {
	const { handle, enableDrillDown = false } = props;
	return (
		<GraphLayoutContainer handle={handle} enableDrillDown={enableDrillDown} />
	);
}

export default GridGraph;
