import { useCallback, useRef, useState } from 'react';

/**
 * One state hook for every controlled-or-uncontrolled primitive in this layer.
 *
 * A component is controlled when the caller passes `value`; then the hook is a
 * pass-through and `setValue` only fires `onChange`. Otherwise the hook owns the
 * state, seeded from `defaultValue`. Which mode is in force is decided once, on
 * first render, and remembered — a component that flips between the two mid-life
 * is a call-site bug, and silently changing behaviour under it hides the bug.
 */
export function useControllable<T>(
	value: T | undefined,
	defaultValue: T,
	onChange?: (next: T) => void,
): [T, (next: T) => void] {
	const controlled = useRef(value !== undefined).current;
	const [uncontrolled, setUncontrolled] = useState<T>(
		value !== undefined ? value : defaultValue,
	);

	const current = controlled ? (value as T) : uncontrolled;

	const set = useCallback(
		(next: T): void => {
			if (!controlled) setUncontrolled(next);
			onChange?.(next);
		},
		[controlled, onChange],
	);

	return [current, set];
}
