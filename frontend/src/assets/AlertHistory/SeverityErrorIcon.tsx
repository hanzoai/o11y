import type { JSX } from 'react';
interface SeverityErrorIconProps {
	width?: number;
	height?: number;
	fill?: string;
	stroke?: string;
	strokeWidth?: string;
}

function SeverityErrorIcon({
	width = 2,
	height = 6,
	fill = 'none',
	stroke = '#F56C87',
	strokeWidth = '1.02083',
}: SeverityErrorIconProps): JSX.Element {
	return (
		<svg
			width={width}
			height={height}
			fill={fill}
			xmlns="http://www.w3.org/2000/svg"
		>
			<path
				d="M1.00781.957845 1.00781 2.99951M1.00781 5.04175H1.00228"
				stroke={stroke}
				strokeWidth={strokeWidth}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</svg>
	);
}

export default SeverityErrorIcon;
