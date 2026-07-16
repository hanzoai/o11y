import { CSSProperties, type JSX } from 'react';
import { Loader } from 'components/ui/icons';
import { Spin, SpinProps } from 'antd';

import { SpinerStyle } from './styles';

function Spinner({
	size = undefined,
	tip = undefined,
	height = undefined,
	style = {},
}: SpinnerProps): JSX.Element {
	return (
		<SpinerStyle height={height} style={style}>
			<Spin
				spinning
				size={size}
				tip={tip}
				indicator={
					<Loader
						className="animate-spin"
						role="img"
						aria-label="loading"
						size="md"
					/>
				}
			/>
		</SpinerStyle>
	);
}

interface SpinnerProps {
	size?: SpinProps['size'];
	tip?: SpinProps['tip'];
	height?: CSSProperties['height'];
	style?: CSSProperties;
}

export default Spinner;
