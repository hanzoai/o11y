import './index.css';
import React from 'react';
import { cn } from '@hanzo/ui/core';
import { SliderThumb } from './slider-thumb';
import { toArray } from './slider-utils';

export interface SliderProps extends Omit<
	React.ComponentPropsWithoutRef<'span'>,
	'onChange' | 'defaultValue'
> {
	value?: number | number[];
	defaultValue?: number | number[];
	/** The lowest selectable value. */
	min?: number;
	/** The highest selectable value. */
	max?: number;
	/** The granularity the value moves in. */
	step?: number;
	/** When true, the slider cannot be moved. */
	disabled?: boolean;
	/**
	 * Tick marks along the track. The key is the numerical value, and the value can be a string, a React node, or an object with label and style.
	 */
	marks?: Record<
		number,
		React.ReactNode | { style?: React.CSSProperties; label: React.ReactNode }
	>;
	/**
	 * Configuration for the tooltip shown above the slider thumb.
	 */
	tooltip?: {
		formatter?: (value: number) => React.ReactNode;
	};
	/**
	 * Callback fired when the value changes during dragging.
	 */
	onChange?: (value: number | number[]) => void;
	/**
	 * Callback fired when `mouseup` or `keyup` happens.
	 */
	onAfterChange?: (value: number | number[]) => void;
	/**
	 * If true, renders a dual-thumb slider for range selection.
	 */
	range?: boolean;
	/**
	 * Custom inline styles for the internal track, range, and thumb elements.
	 */
	styles?: {
		track?: React.CSSProperties;
		range?: React.CSSProperties;
		thumb?: React.CSSProperties;
	};
	/**
	 * Custom CSS class names for the internal track, range, and thumb elements.
	 */
	classNames?: {
		track?: string;
		range?: string;
		thumb?: string;
	};
	/**
	 * Test ID for testing purposes (mapped to data-testid).
	 */
	testId?: string;
}

const clamp = (value: number, min: number, max: number): number =>
	Math.min(Math.max(value, min), max);

/** Snap to the step grid measured from `min`, then keep the result in range. */
const snap = (value: number, min: number, max: number, step: number): number => {
	const snapped = min + Math.round((value - min) / step) * step;
	// Steps rarely divide the range evenly, so round away the float dust the
	// division leaves rather than surfacing 33.800000000000004 to a formatter.
	const decimals = (String(step).split('.')[1] ?? '').length;
	return clamp(Number(snapped.toFixed(decimals)), min, max);
};

/**
 * Slider component for selecting a value or range from a continuous set of values.
 */
const Slider = React.forwardRef<HTMLSpanElement, SliderProps>(
	(
		{
			className,
			marks,
			tooltip,
			onChange,
			onAfterChange,
			range,
			styles: inlineStyles,
			classNames,
			value: controlledValue,
			defaultValue,
			min = 0,
			max = 100,
			step = 1,
			disabled,
			id,
			style,
			testId,
			...props
		},
		ref,
	) => {
		const internalValue = React.useMemo(
			() => toArray(controlledValue),
			[controlledValue],
		);
		const internalDefaultValue = React.useMemo(
			() => toArray(defaultValue),
			[defaultValue],
		);
		const [localValues, setLocalValues] = React.useState<number[]>(
			internalValue || internalDefaultValue || [min],
		);
		const trackRef = React.useRef<HTMLSpanElement | null>(null);
		const [activeIndex, setActiveIndex] = React.useState<number | null>(null);

		React.useEffect(() => {
			if (internalValue !== undefined) {
				setLocalValues(internalValue);
			}
		}, [internalValue]);

		const emit = React.useCallback(
			(newValues: number[], commit: boolean): void => {
				if (internalValue === undefined) {
					setLocalValues(newValues);
				}
				onChange?.(range ? newValues : newValues[0]);
				if (commit) {
					onAfterChange?.(range ? newValues : newValues[0]);
				}
			},
			[internalValue, onChange, onAfterChange, range],
		);

		/** Value under the pointer, read off the track's own box. */
		const valueAt = React.useCallback(
			(clientX: number): number => {
				const rect = trackRef.current?.getBoundingClientRect();
				if (!rect || rect.width === 0) return min;
				const ratio = clamp((clientX - rect.left) / rect.width, 0, 1);
				return snap(min + ratio * (max - min), min, max, step);
			},
			[min, max, step],
		);

		/** Move one thumb, keeping thumbs in order — a range cannot cross itself. */
		const moveThumb = React.useCallback(
			(index: number, next: number, commit: boolean): void => {
				const lower = index > 0 ? localValues[index - 1] : min;
				const upper =
					index < localValues.length - 1 ? localValues[index + 1] : max;
				const bounded = clamp(next, lower, upper);
				if (bounded === localValues[index] && !commit) return;
				const updated = [...localValues];
				updated[index] = bounded;
				emit(updated, commit);
			},
			[localValues, min, max, emit],
		);

		const nearestThumb = React.useCallback(
			(value: number): number => {
				let best = 0;
				let bestDistance = Infinity;
				localValues.forEach((current, index) => {
					const distance = Math.abs(current - value);
					if (distance < bestDistance) {
						bestDistance = distance;
						best = index;
					}
				});
				return best;
			},
			[localValues],
		);

		const startDrag = React.useCallback(
			(event: React.PointerEvent<HTMLSpanElement>, index?: number): void => {
				if (disabled) return;
				const value = valueAt(event.clientX);
				const target = index ?? nearestThumb(value);
				setActiveIndex(target);
				// The pointer belongs to the slider until it is released, so a drag that
				// leaves the element still moves the thumb it grabbed.
				event.currentTarget.setPointerCapture?.(event.pointerId);
				if (index === undefined) moveThumb(target, value, false);
			},
			[disabled, valueAt, nearestThumb, moveThumb],
		);

		React.useEffect(() => {
			if (activeIndex === null) return undefined;
			const onMove = (event: PointerEvent): void =>
				moveThumb(activeIndex, valueAt(event.clientX), false);
			const onUp = (event: PointerEvent): void => {
				moveThumb(activeIndex, valueAt(event.clientX), true);
				setActiveIndex(null);
			};
			window.addEventListener('pointermove', onMove);
			window.addEventListener('pointerup', onUp);
			return (): void => {
				window.removeEventListener('pointermove', onMove);
				window.removeEventListener('pointerup', onUp);
			};
		}, [activeIndex, moveThumb, valueAt]);

		const keyStep = React.useCallback(
			(event: React.KeyboardEvent, index: number): void => {
				const big = (max - min) / 10;
				const delta = {
					ArrowRight: step,
					ArrowUp: step,
					ArrowLeft: -step,
					ArrowDown: -step,
					PageUp: big,
					PageDown: -big,
				}[event.key];
				if (delta !== undefined) {
					event.preventDefault();
					moveThumb(index, snap(localValues[index] + delta, min, max, step), false);
				} else if (event.key === 'Home') {
					event.preventDefault();
					moveThumb(index, min, false);
				} else if (event.key === 'End') {
					event.preventDefault();
					moveThumb(index, max, false);
				}
			},
			[localValues, min, max, step, moveThumb],
		);

		const percentOf = React.useCallback(
			(value: number): number =>
				max === min ? 0 : ((clamp(value, min, max) - min) / (max - min)) * 100,
			[min, max],
		);

		const markList = React.useMemo(() => {
			if (!marks) {
				return [];
			}
			return Object.entries(marks).map(([key, markObj]) => {
				const markVal = Number(key);
				const percent = ((markVal - min) / (max - min)) * 100;
				const isObject =
					typeof markObj === 'object' &&
					markObj !== null &&
					!React.isValidElement(markObj);
				const objMark = markObj as {
					style?: React.CSSProperties;
					label: React.ReactNode;
				};
				return {
					key,
					markVal,
					percent,
					label:
						isObject && 'label' in objMark
							? objMark.label
							: (markObj as React.ReactNode),
					markStyle: isObject && 'style' in objMark ? objMark.style : {},
				};
			});
		}, [marks, min, max]);

		const isMarkActive = React.useCallback(
			(markVal: number): boolean => {
				if (localValues.length === 1) {
					return markVal <= localValues[0];
				}
				return (
					markVal >= localValues[0] && markVal <= localValues[localValues.length - 1]
				);
			},
			[localValues],
		);

		const handleMarkClick = React.useCallback(
			(markVal: number): void => {
				let newValues: number[];
				if (localValues.length === 1) {
					newValues = [markVal];
				} else {
					const lastIndex = localValues.length - 1;
					newValues =
						Math.abs(localValues[0] - markVal) <=
						Math.abs(localValues[lastIndex] - markVal)
							? [markVal, ...localValues.slice(1)]
							: [...localValues.slice(0, lastIndex), markVal];
					newValues = [...newValues].sort((a, b) => a - b);
				}
				emit(newValues, true);
			},
			[localValues, emit],
		);

		const internalId = React.useId();
		const rangeStart = localValues.length > 1 ? percentOf(localValues[0]) : 0;
		const rangeEnd = percentOf(localValues[localValues.length - 1]);

		return (
			<span
				ref={ref}
				id={id}
				style={style}
				data-slot="slider-root"
				data-orientation="horizontal"
				data-disabled={disabled ? '' : undefined}
				data-with-marks={markList.length > 0 ? '' : undefined}
				data-testid={testId}
				className={cn(className)}
				onPointerDown={startDrag}
				{...props}
			>
				<span
					ref={trackRef}
					data-slot="slider-track"
					className={cn(classNames?.track)}
					style={inlineStyles?.track}
				>
					<span
						data-slot="slider-range"
						className={cn(classNames?.range)}
						style={{
							left: `${rangeStart}%`,
							width: `${rangeEnd - rangeStart}%`,
							...inlineStyles?.range,
						}}
					/>
				</span>

				{markList.length > 0 && (
					<span data-slot="slider-dots">
						{markList.map(({ key, markVal, percent }) => (
							<span
								key={`slider-${internalId}-dot-${key}`}
								data-slot="slider-dot"
								data-active={isMarkActive(markVal) ? '' : undefined}
								style={{ left: `${percent}%` }}
							/>
						))}
					</span>
				)}

				{localValues.map((val, index) => (
					<SliderThumb
						// Thumbs are positional (min → max) and never reorder, so the index is a
						// stable identity here.
						// eslint-disable-next-line react/no-array-index-key
						key={`slider-${internalId}-thumb-${index}`}
						value={val}
						min={min}
						max={max}
						percent={percentOf(val)}
						disabled={disabled}
						active={activeIndex === index}
						className={cn(classNames?.thumb)}
						style={inlineStyles?.thumb}
						tooltip={tooltip}
						onPointerDown={(event): void => {
							event.stopPropagation();
							startDrag(event, index);
						}}
						onKeyDown={(event): void => keyStep(event, index)}
						onKeyUp={(): void => emit(localValues, true)}
					/>
				))}

				{markList.length > 0 && (
					<span data-slot="slider-marks">
						{markList.map(({ key, markVal, percent, label, markStyle }) => (
							<button
								key={`slider-${internalId}-mark-${key}`}
								type="button"
								data-slot="slider-mark"
								style={{ left: `${percent}%`, ...markStyle }}
								onPointerDown={(event): void => event.stopPropagation()}
								onClick={(): void => handleMarkClick(markVal)}
							>
								{label}
							</button>
						))}
					</span>
				)}
			</span>
		);
	},
);
Slider.displayName = 'Slider';

export { Slider };
