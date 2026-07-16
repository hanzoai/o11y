import { PANEL_TYPES } from 'constants/queryBuilder';
import { Spline } from 'components/ui/icons';
import { EQueryType } from 'types/common/dashboard';

import QueryTypeTag from '../QueryTypeTag';

import type { JSX } from 'react';

interface IPlotTagProps {
	queryType: EQueryType;
	panelType: PANEL_TYPES;
}

function PlotTag({ queryType, panelType }: IPlotTagProps): JSX.Element | null {
	if (queryType === undefined || panelType === PANEL_TYPES.LIST) {
		return null;
	}

	return (
		<div className="plot-tag">
			<Spline size={14} />
			Plotted with <QueryTypeTag queryType={queryType} />
		</div>
	);
}

export default PlotTag;
