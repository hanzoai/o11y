import { SolidInfoCircle } from 'components/ui/icons';

import {
	DisplayThresholdContainer,
	TypographHeading,
	Typography,
} from './styles';
import { DisplayThresholdProps } from './types';

import type { JSX } from 'react';

function DisplayThreshold({ threshold }: DisplayThresholdProps): JSX.Element {
	return (
		<DisplayThresholdContainer>
			<TypographHeading>Threshold </TypographHeading>
			<Typography>{threshold || <SolidInfoCircle size={16} />}</Typography>
		</DisplayThresholdContainer>
	);
}

export default DisplayThreshold;
