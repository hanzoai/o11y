import React from 'react';

export interface SliderThumbProps {
	value: number;
	min: number;
	max: number;
	/** Position along the track, 0–100. */
	percent: number;
	disabled?: boolean;
	active?: boolean;
	className?: string;
	style?: React.CSSProperties;
	tooltip?: {
		formatter?: (value: number) => React.ReactNode;
	};
	onPointerDown?: (event: React.PointerEvent<HTMLSpanElement>) => void;
	onKeyDown?: (event: React.KeyboardEvent<HTMLSpanElement>) => void;
	onKeyUp?: (event: React.KeyboardEvent<HTMLSpanElement>) => void;
}

/**
 * A single slider thumb. When `tooltip` is supplied the (optionally formatted)
 * value is shown while the thumb is hovered or dragged.
 *
 * The tooltip is a child of the thumb, placed by CSS above it. The thumb is
 * already positioned absolutely at its own value, so "above the thumb, centred"
 * is a static rule — there is nothing here for a positioning engine to compute,
 * and a portal would only add a layer that has to be told where the thumb went.
 */
export const SliderThumb = React.forwardRef<HTMLSpanElement, SliderThumbProps>(
	(
		{
			value,
			min,
			max,
			percent,
			disabled,
			active,
			className,
			style,
			tooltip,
			onPointerDown,
			onKeyDown,
			onKeyUp,
		},
		ref,
	) => {
		const [hovering, setHovering] = React.useState(false);

		return (
			<span
				ref={ref}
				role="slider"
				tabIndex={disabled ? undefined : 0}
				aria-valuemin={min}
				aria-valuemax={max}
				aria-valuenow={value}
				aria-orientation="horizontal"
				aria-disabled={disabled || undefined}
				data-slot="slider-thumb"
				data-disabled={disabled ? '' : undefined}
				className={className}
				style={{
					// The track is inset by half a thumb at each end, so a thumb centred
					// on `percent` of the track sits half a thumb in from the root edge.
					left: `calc(var(--slider-thumb-width, 18px) / 2 + ${percent}% - var(--slider-thumb-width, 18px) * ${percent / 100})`,
					...style,
				}}
				onPointerDown={onPointerDown}
				onKeyDown={onKeyDown}
				onKeyUp={onKeyUp}
				onPointerEnter={(): void => setHovering(true)}
				onPointerLeave={(): void => setHovering(false)}
			>
				{tooltip && (active || hovering) && (
					<span data-slot="slider-tooltip" role="tooltip">
						{tooltip.formatter ? tooltip.formatter(value) : value}
					</span>
				)}
			</span>
		);
	},
);
SliderThumb.displayName = 'SliderThumb';
