import { Span } from 'types/api/trace/getTraceItem';

import { themeColors } from '../constants/theme';

export const colors = Object.values(themeColors.chartcolors);

export function getRandomNumber(min: number, max: number): number {
	return Math.random() * (max - min) + min;
}

const getRandomColor = (): string => {
	const index = parseInt(getRandomNumber(0, colors.length - 1).toString(), 10);
	return colors[index];
};

// eslint-disable-next-line @typescript-eslint/no-inferrable-types
export function hexToRgba(hex: string, alpha: number = 1): string {
	// Create a new local variable to work with
	let hexColor = hex;

	// Ensure the hex string has a "#" at the start
	if (hexColor.charAt(0) === '#') {
		hexColor = hexColor.slice(1);
	}

	// Check if it's a shorthand hex code (e.g., #FFF)
	if (hexColor.length === 3) {
		const r = hexColor.charAt(0);
		const g = hexColor.charAt(1);
		const b = hexColor.charAt(2);
		hexColor = r + r + g + g + b + b;
	}

	// Parse the r, g, b values
	const bigint = parseInt(hexColor, 16);
	const r = (bigint >> 16) & 255;
	const g = (bigint >> 8) & 255;
	const b = bigint & 255;

	return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

export const HANZO_UI_COLOR_HEX = 'o11y_ui_color_hex';

export const spanServiceNameToColorMapping = (
	spans: Span[],
): { [key: string]: string } => {
	const allServiceMap = new Map<string, string | undefined>();

	spans.forEach((spanItem) => {
		const o11yUiColorKeyIndex = spanItem[7].findIndex(
			(span) => span === HANZO_UI_COLOR_HEX,
		);

		allServiceMap.set(
			spanItem[3],
			o11yUiColorKeyIndex === -1
				? undefined
				: spanItem[8][o11yUiColorKeyIndex],
		);
	});

	const serviceToColorMap: { [key: string]: string } = {};

	Array.from(allServiceMap).forEach(([serviceName, o11yColor], idx) => {
		serviceToColorMap[`${serviceName}`] =
			o11yColor || colors[idx % colors.length];
	});

	return serviceToColorMap;
};

export default getRandomColor;
