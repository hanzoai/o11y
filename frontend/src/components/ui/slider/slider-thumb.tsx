import * as SliderPrimitive from '@radix-ui/react-slider';
import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import React from 'react';

export interface SliderThumbProps {
	value: number;
	className?: string;
	style?: React.CSSProperties;
	tooltip?: {
		formatter?: (value: number) => React.ReactNode;
	};
}

/**
 * A single slider thumb. When `tooltip` is supplied the thumb is wrapped in a
 * tooltip that surfaces the (optionally formatted) value while the thumb is
 * hovered or dragged — reproducing Periscope's thumb tooltip behavior.
 */
export function SliderThumb({
	value,
	className,
	style,
	tooltip,
}: SliderThumbProps): React.ReactElement {
	const [isDragging, setIsDragging] = React.useState(false);
	const [isHovering, setIsHovering] = React.useState(false);

	React.useEffect(() => {
		if (!isDragging) {
			return undefined;
		}
		const handlePointerUp = (): void => setIsDragging(false);
		window.addEventListener('pointerup', handlePointerUp);
		return (): void => window.removeEventListener('pointerup', handlePointerUp);
	}, [isDragging]);

	const thumb = (
		<SliderPrimitive.Thumb
			data-slot="slider-thumb"
			className={className}
			style={style}
			onPointerDown={(): void => setIsDragging(true)}
			onPointerEnter={(): void => setIsHovering(true)}
			onPointerLeave={(): void => setIsHovering(false)}
		/>
	);

	if (!tooltip) {
		return thumb;
	}

	return (
		<TooltipPrimitive.Provider delayDuration={0}>
			<TooltipPrimitive.Root open={isDragging || isHovering}>
				<TooltipPrimitive.Trigger asChild>{thumb}</TooltipPrimitive.Trigger>
				<TooltipPrimitive.Portal>
					<TooltipPrimitive.Content data-slot="slider-tooltip" side="top" sideOffset={6}>
						{tooltip.formatter ? tooltip.formatter(value) : value}
					</TooltipPrimitive.Content>
				</TooltipPrimitive.Portal>
			</TooltipPrimitive.Root>
		</TooltipPrimitive.Provider>
	);
}
