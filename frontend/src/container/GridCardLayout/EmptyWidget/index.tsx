import { Typography } from 'components/ui/typography';

import { Container } from './styles';

import type { JSX } from 'react';

function EmptyWidget(): JSX.Element {
	return (
		<Container>
			<Typography.Text>
				Click one of the widget types above (Time Series / Value) to add here
			</Typography.Text>
		</Container>
	);
}

export default EmptyWidget;
