import './index.css';
import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import * as React from 'react';

import { cn } from '../lib/utils';

export type TooltipProviderProps = {
	/**
	 * The children of the tooltip provider.
	 */
	children: React.ReactNode;
	/**
	 * The duration from when the pointer enters the trigger until the tooltip gets opened.
	 * @defaultValue 700
	 */
	delayDuration?: number;
	/**
	 * How much time a user has to enter another trigger without incurring a delay again.
	 * @defaultValue 300
	 */
	skipDelayDuration?: number;
	/**
	 * When `true`, trying to hover the content will result in the tooltip closing as the pointer leaves the trigger.
	 * @defaultValue false
	 */
	disableHoverableContent?: boolean;
	/**
	 * The test id of the tooltip provider.
	 */
	testId?: string;
};

/**
 * Wraps your app (or a section of it) to provide shared configuration for all tooltips.
 */
function TooltipProvider({ delayDuration = 0, testId, ...props }: TooltipProviderProps) {
	return (
		<TooltipPrimitive.Provider
			data-slot="tooltip-provider"
			data-testid={testId}
			delayDuration={delayDuration}
			{...props}
		/>
	);
}

export type TooltipRootProps = {
	/**
	 * The tooltip trigger and content elements.
	 */
	children?: React.ReactNode;
	/**
	 * The controlled open state of the tooltip.
	 */
	open?: boolean;
	/**
	 * The open state of the tooltip when it is initially rendered.
	 */
	defaultOpen?: boolean;
	/**
	 * Event handler called when the open state of the tooltip changes.
	 */
	onOpenChange?: (open: boolean) => void;
	/**
	 * The duration from when the pointer enters the trigger until the tooltip gets opened. This will
	 * override the prop with the same name passed to Provider.
	 * @defaultValue 700
	 */
	delayDuration?: number;
	/**
	 * When `true`, trying to hover the content will result in the tooltip closing as the pointer leaves the trigger.
	 * @defaultValue false
	 */
	disableHoverableContent?: boolean;
	/**
	 * The test id of the tooltip root.
	 */
	testId?: string;
};

/**
 * Root component that manages the open state and accessibility wiring for a tooltip.
 */
function TooltipRoot({ testId, ...props }: TooltipRootProps) {
	return <TooltipPrimitive.Root data-slot="tooltip" data-testid={testId} {...props} />;
}

export type TooltipTriggerProps = {
	/**
	 * The children of the tooltip trigger.
	 */
	children?: React.ReactNode;
	/**
	 * When true, merges props onto the child element instead of rendering a wrapper.
	 */
	asChild?: boolean;
	/**
	 * The test id of the tooltip trigger.
	 */
	testId?: string;
};

/**
 * The element that triggers the tooltip to open on hover. Use with `asChild` to delegate
 * to a child element (e.g. a Button).
 */
const TooltipTrigger = React.forwardRef<HTMLButtonElement, TooltipTriggerProps>(
	({ testId, ...props }, ref) => (
		<TooltipPrimitive.Trigger
			ref={ref}
			data-slot="tooltip-trigger"
			data-testid={testId}
			{...props}
		/>
	)
);
TooltipTrigger.displayName = 'TooltipTrigger';

type OriginalTooltipContentProps = React.ComponentProps<typeof TooltipPrimitive.Content>;

export type TooltipContentProps = {
	/**
	 * The preferred side of the trigger to render against when open. Will be reversed when collisions occur and avoidCollisions is enabled.
	 */
	side?: OriginalTooltipContentProps['side'];
	/**
	 * The distance in pixels from the trigger.
	 */
	sideOffset?: number;
	/**
	 * The preferred alignment against the trigger. May change when collisions occur.
	 */
	align?: OriginalTooltipContentProps['align'];
	/**
	 * An offset in pixels from the "start" or "end" alignment options.
	 */
	alignOffset?: number;
	/**
	 * The padding between the arrow and the edges of the content. If your content has border-radius, this will prevent it from overflowing the corners.
	 */
	arrowPadding?: number;
	/**
	 * When true, overrides the side and align preferences to prevent collisions with boundary edges.
	 */
	avoidCollisions?: boolean;
	/**
	 * The element used as the collision boundary. By default this is the viewport, though you can provide additional element(s) to be included in this check.
	 */
	collisionBoundary?: OriginalTooltipContentProps['collisionBoundary'];
	/**
	 * The distance in pixels from the boundary edges where collision detection should occur. Accepts a number (same for all sides), or a partial padding object, for example: { top: 20, left: 20 }.
	 */
	collisionPadding?: OriginalTooltipContentProps['collisionPadding'];
	/**
	 * The sticky behavior on the align axis. "partial" will keep the content in the boundary as long as the trigger is at least partially in the boundary whilst "always" will keep the content in the boundary regardless.
	 */
	sticky?: 'partial' | 'always';
	/**
	 * Whether to hide the content when the trigger becomes fully occluded.
	 */
	hideWhenDetached?: boolean;
	/**
	 * The strategy used to update the position of the content. "optimized" will use ResizeObserver to
	 * only update when necessary; "always" will update on every frame.
	 * @defaultValue 'optimized'
	 */
	updatePositionStrategy?: 'optimized' | 'always';
	/**
	 * Used to force mounting when more control is needed. Useful when
	 * controlling animation with React animation libraries.
	 */
	forceMount?: true;
	/**
	 * A more descriptive label for accessibility purpose
	 */
	'aria-label'?: string;
	/**
	 * Event handler called when the escape key is down.
	 * Can be prevented.
	 */
	onEscapeKeyDown?: OriginalTooltipContentProps['onEscapeKeyDown'];
	/**
	 * Event handler called when the a `pointerdown` event happens outside of the `Tooltip`.
	 * Can be prevented.
	 */
	onPointerDownOutside?: OriginalTooltipContentProps['onPointerDownOutside'];
	/**
	 * Whether to show the arrow.
	 */
	arrow?: boolean;
	/**
	 * Whether to render in a portal. Set to false when inside modals/dialogs.
	 * @default true
	 */
	withPortal?: boolean;
	/**
	 * The test id of the tooltip content.
	 */
	testId?: string;
} & Pick<React.ComponentProps<'div'>, 'id' | 'className' | 'style' | 'children'>;

const TooltipContentInner = React.forwardRef<HTMLDivElement, TooltipContentProps>(
	({ className, sideOffset = 4, testId, children, arrow = false, ...props }, ref) => (
		<TooltipPrimitive.Content
			ref={ref}
			data-slot="tooltip-content"
			data-testid={testId}
			sideOffset={arrow ? 0 : sideOffset}
			className={cn(className)}
			{...props}
		>
			{children}
			{arrow && (
				<TooltipPrimitive.Arrow asChild data-slot="tooltip-arrow">
					<svg width={10} height={5} viewBox="0 0 30 10" preserveAspectRatio="none">
						<path d="M 0,0 L 15,10 L 30,0" data-slot="tooltip-arrow-path" />
					</svg>
				</TooltipPrimitive.Arrow>
			)}
		</TooltipPrimitive.Content>
	)
);
TooltipContentInner.displayName = 'TooltipContentInner';

/**
 * The content of the tooltip. Supports positioning via `side`, `align`, and collision detection.
 * Set `withPortal={false}` when inside modals/dialogs to avoid z-index issues.
 */
const TooltipContent = React.forwardRef<HTMLDivElement, TooltipContentProps>(
	({ withPortal = true, ...props }, ref) => {
		if (withPortal) {
			return (
				<TooltipPrimitive.Portal>
					<TooltipContentInner ref={ref} {...props} />
				</TooltipPrimitive.Portal>
			);
		}
		return <TooltipContentInner ref={ref} {...props} />;
	}
);
TooltipContent.displayName = 'TooltipContent';

export type TooltipSimpleProps = {
	/**
	 * The content of the tooltip.
	 */
	title: React.ReactNode;
	/**
	 * Whether to show the arrow.
	 * @default false
	 */
	arrow?: boolean;
	/**
	 * The preferred side of the trigger to render against when open.
	 * @default 'top'
	 */
	side?: TooltipContentProps['side'];
	/**
	 * The preferred alignment against the trigger.
	 * @default 'center'
	 */
	align?: TooltipContentProps['align'];
	/**
	 * The distance in pixels from the trigger.
	 * @default 4
	 */
	sideOffset?: number;
	/**
	 * An offset in pixels from the "start" or "end" alignment options.
	 */
	alignOffset?: number;
	/**
	 * When true, overrides the side and align preferences to prevent collisions with boundary edges.
	 * @default true
	 */
	avoidCollisions?: boolean;
	/**
	 * Whether to render in a portal. Set to false when inside modals/dialogs.
	 * @default true
	 */
	withPortal?: boolean;
	/**
	 * Additional props to pass to TooltipContent.
	 */
	tooltipContentProps?: Omit<
		TooltipContentProps,
		'side' | 'align' | 'sideOffset' | 'alignOffset' | 'avoidCollisions' | 'arrow' | 'withPortal' | 'children'
	>;
	/**
	 * The test id of the tooltip.
	 */
	testId?: string;
	/**
	 * The trigger element.
	 */
	children: React.ReactNode;
} & Omit<TooltipRootProps, 'children'>;

/**
 * Simple tooltip preset. Wraps a trigger element and shows a tooltip on hover.
 */
const TooltipSimple = React.forwardRef<HTMLButtonElement, TooltipSimpleProps>(
	(
		{
			title,
			arrow = false,
			side,
			align,
			sideOffset,
			alignOffset,
			avoidCollisions,
			withPortal = true,
			tooltipContentProps,
			testId,
			children,
			...rootProps
		},
		ref
	) => {
		return (
			<TooltipRoot data-testid={testId} {...rootProps}>
				<TooltipTrigger ref={ref} asChild>
					{children}
				</TooltipTrigger>
				<TooltipContent
					arrow={arrow}
					side={side}
					align={align}
					sideOffset={sideOffset}
					alignOffset={alignOffset}
					avoidCollisions={avoidCollisions}
					withPortal={withPortal}
					{...tooltipContentProps}
				>
					{title}
				</TooltipContent>
			</TooltipRoot>
		);
	}
);
TooltipSimple.displayName = 'TooltipSimple';

export { TooltipProvider, TooltipRoot, TooltipTrigger, TooltipContent, TooltipSimple };
