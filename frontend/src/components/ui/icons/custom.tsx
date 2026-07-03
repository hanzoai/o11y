/**
 * Custom (non-lucide) icons used across the app.
 *
 * These 8 glyphs existed in the previous icon set but have no lucide-react
 * equivalent, so they are vendored here as plain SVG React components with a
 * lucide-compatible prop surface ({ size, color, strokeWidth, className }).
 * Every other icon in the app comes straight from lucide-react (see ./index).
 */
import type { SVGProps } from 'react';
import { forwardRef } from 'react';

export interface IconProps extends Omit<SVGProps<SVGSVGElement>, 'ref'> {
	size?: number | string;
	color?: string;
	strokeWidth?: number | string;
}

type SolidSpec = {
	name: string;
	viewBox?: string;
	fill?: string;
	body: JSX.Element;
};

function makeIcon({ name, viewBox = '0 0 16 16', fill = 'none', body }: SolidSpec) {
	const Icon = forwardRef<SVGSVGElement, IconProps>(
		({ size = 16, color = 'currentColor', className, ...rest }, ref) => (
			<svg
				ref={ref}
				width={size}
				height={size}
				viewBox={viewBox}
				fill={fill}
				xmlns="http://www.w3.org/2000/svg"
				className={className}
				style={{ color, ...(rest.style as object) }}
				{...rest}
			>
				{body}
			</svg>
		),
	);
	Icon.displayName = name;
	return Icon;
}

export const SolidInfoCircle = makeIcon({
	name: 'SolidInfoCircle',
	body: (
		<>
			<path
				d="M8 14.667A6.667 6.667 0 1 0 8 1.333a6.667 6.667 0 0 0 0 13.334Z"
				fill="currentColor"
			/>
			<path
				d="M8 11.333v-4H6.333M8 4.667h.007"
				stroke="#fff"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});

export const SolidXCircle = makeIcon({
	name: 'SolidXCircle',
	body: (
		<>
			<path
				d="M8 14.667A6.667 6.667 0 1 0 8 1.333a6.667 6.667 0 0 0 0 13.334Z"
				fill="currentColor"
			/>
			<path
				d="m10 6-4 4M6 6l4 4"
				stroke="#fff"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});

export const SolidCheckCircle2 = makeIcon({
	name: 'SolidCheckCircle2',
	body: (
		<>
			<path
				d="M8 14.667A6.667 6.667 0 1 0 8 1.334a6.667 6.667 0 0 0 0 13.333Z"
				fill="currentColor"
			/>
			<path
				d="m4 8.017 2.367 2.316L12 5.667"
				stroke="#fff"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});

export const SolidAlertTriangle = makeIcon({
	name: 'SolidAlertTriangle',
	body: (
		<>
			<path
				d="M6.86 2.573 1.215 12a1.333 1.333 0 0 0 1.14 2h11.293a1.333 1.333 0 0 0 1.14-2L9.14 2.573a1.333 1.333 0 0 0-2.28 0Z"
				fill="currentColor"
			/>
			<path
				d="M8 6v2.667M8 11.333h.007"
				stroke="#fff"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});

export const SolidAlertOctagon = makeIcon({
	name: 'SolidAlertOctagon',
	body: (
		<>
			<path
				d="M5.24 1.333h5.52l3.906 3.907v5.52l-3.906 3.907H5.24L1.333 10.76V5.24L5.24 1.333Z"
				fill="currentColor"
			/>
			<path
				d="M8 5.333V8M8 10.667h.007"
				stroke="#fff"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});

export const SolidGoogle = makeIcon({
	name: 'SolidGoogle',
	viewBox: '0 0 512 512',
	fill: 'currentColor',
	body: (
		<path d="M500 261.8C500 403.3 403.1 504 260 504 122.8 504 12 393.2 12 256S122.8 8 260 8c66.8 0 123 24.5 166.3 64.9l-67.5 64.9C270.5 52.6 106.3 116.6 106.3 256c0 86.5 69.1 156.6 153.7 156.6 98.2 0 135-70.4 140.8-106.9H260v-85.3h236.1c2.3 12.7 3.9 24.9 3.9 41.4z" />
	),
});

export const EyeOpen = makeIcon({
	name: 'EyeOpen',
	body: (
		<>
			<path
				d="M1.333 9.333s2-4.666 6.667-4.666c4.666 0 6.666 4.666 6.666 4.666"
				stroke="currentColor"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<path
				d="M8 11.333a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z"
				stroke="currentColor"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});

export const Histogram = makeIcon({
	name: 'Histogram',
	body: (
		<>
			<path
				d="M8.667 2H7.333C6.597 2 6 2.672 6 3.5V14h4V3.5C10 2.672 9.403 2 8.667 2Z"
				stroke="currentColor"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<path
				d="M10.333 7.333h2.334c.736 0 1.333.664 1.333 1.482v3.704c0 .818-.597 1.481-1.333 1.481H8M5.667 5.333H3.333C2.597 5.333 2 6.196 2 7.26v4.815C2 13.138 2.597 14 3.333 14H8"
				stroke="currentColor"
				strokeWidth={1.333}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</>
	),
});
