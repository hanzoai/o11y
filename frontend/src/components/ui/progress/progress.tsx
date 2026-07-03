import './index.css';
import * as ProgressPrimitive from '@radix-ui/react-progress';
import React from 'react';
import { cn } from '../lib/utils';

export interface ProgressProps
	extends React.ComponentPropsWithoutRef<typeof ProgressPrimitive.Root> {
	/**
	 * The completion value from 0 to 100.
	 * @defaultValue 0
	 */
	percent?: number;
	/**
	 * If provided, divides the progress bar into equal visual segments instead of a continuous bar.
	 */
	steps?: number;
	/**
	 * Controls the edge styling of the progress indicator.
	 * @defaultValue 'butt'
	 */
	strokeLinecap?: 'butt' | 'round';
	/**
	 * A CSS color value to dynamically override the indicator's background color.
	 */
	strokeColor?: string;
	/**
	 * The size of the progress bar.
	 * @defaultValue 'default'
	 */
	size?: 'small' | 'default';
	/**
	 * If true, renders the percent value as text next to the progress bar.
	 * @defaultValue false
	 */
	showInfo?: boolean;
	/**
	 * If 'active', applies a subtle striped animation to the progress bar.
	 * @defaultValue 'normal'
	 */
	status?: 'normal' | 'active';
	/**
	 * Test ID for the progress bar.
	 */
	testId?: string;
	/**
	 * A unique identifier for the progress bar.
	 */
	id?: string;
	/**
	 * Inline styles applied to the progress wrapper.
	 */
	style?: React.CSSProperties;
}

/**
 * Displays a progress bar indicating the completion percentage of a task or process.
 * Supports different sizes, line cap styles, step dividers, and an animated active state.
 */
const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
	(
		{
			className,
			percent = 0,
			steps,
			strokeLinecap = 'butt',
			strokeColor,
			size = 'default',
			showInfo = false,
			status = 'normal',
			testId,
			id,
			style,
			...props
		},
		ref
	) => {
		const internalId = React.useId();
		const clampedPercent = Math.min(Math.max(percent, 0), 100);
		const stepPositions = React.useMemo(() => {
			if (!steps || steps <= 1) {
				return [];
			}
			const total = steps;
			return Array.from({ length: total - 1 }, (_, i) => ((i + 1) * 100) / total);
		}, [steps]);
		return (
			<div
				data-slot="progress-wrapper"
				className={cn(className)}
				data-testid={testId}
				id={id}
				style={style}
			>
				<ProgressPrimitive.Root
					ref={ref}
					data-slot="progress"
					data-size={size}
					data-linecap={strokeLinecap}
					value={clampedPercent}
					{...props}
				>
					<ProgressPrimitive.Indicator
						data-slot="progress-indicator"
						data-status={status}
						style={{
							transform: `translateX(-${100 - clampedPercent}%)`,
							backgroundColor: strokeColor,
						}}
					/>
					{stepPositions.length > 0 ? (
						<div data-slot="progress-step-dividers" aria-hidden="true">
							{stepPositions.map((left) => (
								<div
									key={`progress-step-divider-${internalId}-${left}`}
									data-slot="progress-step-divider"
									style={{ left: `${left}%` }}
								/>
							))}
						</div>
					) : null}
				</ProgressPrimitive.Root>
				{showInfo && <span data-slot="progress-info">{clampedPercent}%</span>}
			</div>
		);
	}
);
Progress.displayName = 'Progress';

export { Progress };
