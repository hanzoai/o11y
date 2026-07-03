import './index.css';
import * as SliderPrimitive from '@radix-ui/react-slider';
import React from 'react';
import { cn } from '../lib/utils';
import { SliderThumb } from './slider-thumb';
import { toArray } from './slider-utils';

export interface SliderProps
	extends Omit<
		React.ComponentPropsWithoutRef<typeof SliderPrimitive.Root>,
		'onChange' | 'value' | 'defaultValue'
	> {
	value?: number | number[];
	defaultValue?: number | number[];
	/**
	 * Tick marks along the track. The key is the numerical value, and the value can be a string, a React node, or an object with label and style.
	 */
	marks?: Record<
		number,
		React.ReactNode | { style?: React.CSSProperties; label: React.ReactNode }
	>;
	/**
	 * Configuration for the tooltip wrapped around the slider thumb.
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
	/**
	 * Unique identifier for the slider root element.
	 */
	id?: string;
	/**
	 * Inline style for the slider root element.
	 */
	style?: React.CSSProperties;
}

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
			id,
			style,
			testId,
			...props
		},
		ref
	) => {
		const internalValue = React.useMemo(() => toArray(controlledValue), [controlledValue]);
		const internalDefaultValue = React.useMemo(() => toArray(defaultValue), [defaultValue]);
		const [localValues, setLocalValues] = React.useState<number[]>(
			internalValue || internalDefaultValue || [min]
		);

		React.useEffect(() => {
			if (internalValue !== undefined) {
				setLocalValues(internalValue);
			}
		}, [internalValue]);

		const handleValueChange = React.useCallback(
			(newValues: number[]): void => {
				if (internalValue === undefined) {
					setLocalValues(newValues);
				}
				if (onChange) {
					onChange(range ? newValues : newValues[0]);
				}
			},
			[internalValue, onChange, range]
		);

		const handleValueCommit = React.useCallback(
			(newValues: number[]): void => {
				if (onAfterChange) {
					onAfterChange(range ? newValues : newValues[0]);
				}
			},
			[onAfterChange, range]
		);

		const markList = React.useMemo(() => {
			if (!marks) {
				return [];
			}
			return Object.entries(marks).map(([key, markObj]) => {
				const markVal = Number(key);
				const percent = ((markVal - min) / (max - min)) * 100;
				const isObject =
					typeof markObj === 'object' && markObj !== null && !React.isValidElement(markObj);
				const objMark = markObj as { style?: React.CSSProperties; label: React.ReactNode };
				return {
					key,
					markVal,
					percent,
					label: isObject && 'label' in objMark ? objMark.label : (markObj as React.ReactNode),
					markStyle: isObject && 'style' in objMark ? objMark.style : {},
				};
			});
		}, [marks, min, max]);

		const isMarkActive = React.useCallback(
			(markVal: number): boolean => {
				if (localValues.length === 1) {
					return markVal <= localValues[0];
				}
				return markVal >= localValues[0] && markVal <= localValues[localValues.length - 1];
			},
			[localValues]
		);

		const handleMarkClick = React.useCallback(
			(markVal: number): void => {
				let newValues: number[];
				if (localValues.length === 1) {
					newValues = [markVal];
				} else {
					const lastIndex = localValues.length - 1;
					newValues =
						Math.abs(localValues[0] - markVal) <= Math.abs(localValues[lastIndex] - markVal)
							? [markVal, ...localValues.slice(1)]
							: [...localValues.slice(0, lastIndex), markVal];
					newValues = [...newValues].sort((a, b) => a - b);
				}
				if (internalValue === undefined) {
					setLocalValues(newValues);
				}
				if (onChange) {
					onChange(range ? newValues : newValues[0]);
				}
				if (onAfterChange) {
					onAfterChange(range ? newValues : newValues[0]);
				}
			},
			[localValues, internalValue, onChange, onAfterChange, range]
		);

		const internalId = React.useId();

		return (
			<SliderPrimitive.Root
				ref={ref}
				id={id}
				style={style}
				data-slot="slider-root"
				data-with-marks={markList.length > 0 ? '' : undefined}
				data-testid={testId}
				min={min}
				max={max}
				value={localValues}
				defaultValue={internalDefaultValue}
				onValueChange={handleValueChange}
				onValueCommit={handleValueCommit}
				className={cn(className)}
				{...props}
			>
				<SliderPrimitive.Track
					data-slot="slider-track"
					className={cn(classNames?.track)}
					style={inlineStyles?.track}
				>
					<SliderPrimitive.Range
						data-slot="slider-range"
						className={cn(classNames?.range)}
						style={inlineStyles?.range}
					/>
				</SliderPrimitive.Track>

				{markList.length > 0 && (
					<div data-slot="slider-dots">
						{markList.map(({ key, markVal, percent }) => (
							<span
								key={`slider-${internalId}-dot-${key}`}
								data-slot="slider-dot"
								data-active={isMarkActive(markVal) ? '' : undefined}
								style={{ left: `${percent}%` }}
							/>
						))}
					</div>
				)}

				{localValues.map((val, index) => (
					<SliderThumb
						// Thumbs are positional (min → max) and never reorder, so the index is a
						// stable identity here.
						// eslint-disable-next-line react/no-array-index-key
						key={`slider-${internalId}-thumb-${index}`}
						value={val}
						className={cn(classNames?.thumb)}
						style={inlineStyles?.thumb}
						tooltip={tooltip}
					/>
				))}

				{markList.length > 0 && (
					<div data-slot="slider-marks">
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
					</div>
				)}
			</SliderPrimitive.Root>
		);
	}
);
Slider.displayName = 'Slider';

export { Slider };
