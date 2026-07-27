/**
 * Normalizes a scalar-or-array slider value into an array (or undefined when the
 * value is not provided). Mirrors the internal `toArray` helper Periscope uses so
 * a single-thumb `number` and a range `number[]` share one code path.
 */
export function toArray(val?: number | number[]): number[] | undefined {
	if (Array.isArray(val)) {
		return val;
	}
	return val !== undefined ? [val] : undefined;
}
