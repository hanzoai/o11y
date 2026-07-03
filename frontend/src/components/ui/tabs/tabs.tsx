import './index.css';
import * as React from 'react';
import * as TabsPrimitive from '@radix-ui/react-tabs';
import { Lock } from 'lucide-react';
import { cn } from '../lib/utils';

export type TabVariants = 'primary' | 'secondary';

export type TabItemProps = {
	/** Unique identifier for the tab item. */
	key: string;
	/** The label content displayed in the tab trigger. */
	label: React.ReactNode;
	/** The content displayed when the tab is active. */
	children: React.ReactNode;
	/** When true, prevents the user from interacting with the tab. */
	disabled?: boolean;
	/** Tooltip message shown when the tab is disabled. */
	disabledReason?: string;
	/** Icon displayed before the tab label. */
	prefixIcon?: React.ReactNode;
	/** Icon displayed after the tab label. */
	suffixIcon?: React.ReactNode;
};

export type TabsRootProps = Pick<
	React.ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The testId associated with the tabs root. */
	testId?: string;
	/** The direction of navigation when using keyboard. */
	dir?: 'ltr' | 'rtl';
	/** The value of the tab that should be active when initially rendered. */
	defaultValue?: string;
	/** The controlled value of the tab to activate. */
	value?: string;
	/** Event handler called when the active tab changes. */
	onValueChange?: (value: string) => void;
	/** The orientation of the tabs. */
	orientation?: 'horizontal' | 'vertical';
	/**
	 * When automatic, tabs are activated when receiving focus.
	 * When manual, tabs are activated when clicked.
	 * @default 'automatic'
	 */
	activationMode?: 'automatic' | 'manual';
};

/**
 * Root container for primitive tabs composition.
 * Use this when you need full control over the tabs structure.
 */
export const TabsRoot = React.forwardRef<HTMLDivElement, TabsRootProps>(
	({ className, testId, ...props }, ref) => (
		<TabsPrimitive.Root
			ref={ref}
			data-slot="tabs"
			className={cn('tabs', className)}
			data-testid={testId}
			{...props}
		/>
	)
);
TabsRoot.displayName = 'TabsRoot';

export type TabsListProps = Pick<
	React.ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The testId associated with the tabs list. */
	testId?: string;
	/** The visual style variant of the tabs list. */
	variant?: TabVariants;
	/** When true, keyboard navigation will loop from last tab to first, and vice versa. */
	loop?: boolean;
};

/**
 * Container for tab triggers that provides navigation and styling.
 * In the primary variant it renders animated hover/active sliders.
 */
export const TabsList = React.forwardRef<HTMLDivElement, TabsListProps>(
	({ className, variant = 'primary', children, testId, ...props }, ref) => {
		const listRef = React.useRef<HTMLDivElement | null>(null);
		const activeSliderRef = React.useRef<HTMLDivElement | null>(null);
		const hoverSliderRef = React.useRef<HTMLDivElement | null>(null);
		React.useImperativeHandle(ref, () => listRef.current as HTMLDivElement);

		const updateSliderPosition = React.useCallback(
			(slider: HTMLDivElement | null, trigger: Element | null) => {
				if (!slider || !trigger || !listRef.current) {
					if (slider) slider.style.opacity = '0';
					return;
				}
				const listRect = listRef.current.getBoundingClientRect();
				const triggerRect = trigger.getBoundingClientRect();
				const offset = triggerRect.left - listRect.left;
				slider.style.transform = `translateX(${offset}px)`;
				slider.style.width = `${triggerRect.width}px`;
				slider.style.opacity = '1';
			},
			[]
		);

		const updateActiveSlider = React.useCallback(() => {
			if (variant !== 'primary' || !listRef.current) return;
			const activeTrigger = listRef.current.querySelector(
				'[data-slot="tabs-trigger"][data-state="active"]'
			);
			updateSliderPosition(activeSliderRef.current, activeTrigger);
		}, [variant, updateSliderPosition]);

		React.useEffect(() => {
			if (variant !== 'primary') return undefined;
			requestAnimationFrame(updateActiveSlider);
			const list = listRef.current;
			if (!list) return undefined;
			const observer = new MutationObserver((mutations) => {
				for (const mutation of mutations) {
					if (mutation.type === 'attributes' && mutation.attributeName === 'data-state') {
						updateActiveSlider();
						break;
					}
				}
			});
			observer.observe(list, {
				attributes: true,
				attributeFilter: ['data-state'],
				subtree: true,
			});
			return (): void => observer.disconnect();
		}, [variant, updateActiveSlider]);

		React.useEffect(() => {
			if (variant !== 'primary') return undefined;
			const handleResize = (): void => updateActiveSlider();
			window.addEventListener('resize', handleResize);
			return (): void => window.removeEventListener('resize', handleResize);
		}, [variant, updateActiveSlider]);

		const handleMouseOver = React.useCallback(
			(e: React.MouseEvent) => {
				if (variant !== 'primary') return;
				const trigger = (e.target as HTMLElement).closest('[data-slot="tabs-trigger"]');
				if (trigger) updateSliderPosition(hoverSliderRef.current, trigger);
			},
			[variant, updateSliderPosition]
		);

		const handleMouseLeave = React.useCallback(() => {
			if (variant !== 'primary') return;
			const slider = hoverSliderRef.current;
			if (slider) slider.style.opacity = '0';
		}, [variant]);

		return (
			<div data-slot="tabs-list-wrapper" data-variant={variant}>
				{variant === 'secondary' && (
					<div data-slot="tabs-border-spacer" />
				)}
				<TabsPrimitive.List
					ref={listRef}
					data-slot="tabs-list"
					className={cn('tabs-list', className)}
					data-variant={variant}
					data-testid={testId}
					onMouseOver={variant === 'primary' ? handleMouseOver : undefined}
					onMouseLeave={variant === 'primary' ? handleMouseLeave : undefined}
					{...props}
				>
					{children}
				</TabsPrimitive.List>
				{variant === 'secondary' ? (
					<div data-slot="tabs-border-spacer" data-grow />
				) : (
					<>
						<div
							ref={hoverSliderRef}
							data-slot="tabs-hover-slider"
							style={{ height: '28px', opacity: 0 }}
						/>
						<div ref={activeSliderRef} data-slot="tabs-active-slider" style={{ opacity: 0 }} />
					</>
				)}
			</div>
		);
	}
);
TabsList.displayName = 'TabsList';

export type TabsTriggerProps = Pick<
	React.ComponentPropsWithoutRef<'button'>,
	'id' | 'className' | 'style' | 'children' | 'onMouseEnter' | 'onMouseDown' | 'onMouseLeave' | 'title'
> & {
	/** The testId associated with the tabs trigger. */
	testId?: string;
	/** The unique value that associates the trigger with a content panel. */
	value: string;
	/** When true, prevents the user from interacting with the tab. */
	disabled?: boolean;
	/** The visual style variant of the trigger. */
	variant?: TabVariants;
};

/**
 * Interactive button that activates its associated tab content panel.
 */
export const TabsTrigger = React.forwardRef<HTMLButtonElement, TabsTriggerProps>(
	({ className, children, variant = 'primary', disabled, testId, ...props }, ref) => (
		<TabsPrimitive.Trigger
			ref={ref}
			data-slot="tabs-trigger"
			data-variant={variant}
			data-testid={testId}
			className={cn('tabs-trigger', className)}
			disabled={disabled}
			{...props}
		>
			{children}
		</TabsPrimitive.Trigger>
	)
);
TabsTrigger.displayName = 'TabsTrigger';

export type TabsContentProps = Pick<
	React.ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The testId associated with the tabs content. */
	testId?: string;
	/** The unique value that associates the content with a trigger. */
	value: string;
	/** When true, content is kept mounted in the DOM when inactive. */
	forceMount?: true;
};

/**
 * Container for the content associated with a tab trigger.
 */
export const TabsContent = React.forwardRef<HTMLDivElement, TabsContentProps>(
	({ className, testId, ...props }, ref) => (
		<TabsPrimitive.Content
			ref={ref}
			data-slot="tabs-content"
			className={cn('tabs-content', className)}
			data-testid={testId}
			{...props}
		/>
	)
);
TabsContent.displayName = 'TabsContent';

export type TabsProps = Pick<
	React.ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The testId associated with the tabs. */
	testId?: string;
	/** Array of tab items to render. */
	items: TabItemProps[];
	/**
	 * The visual style variant of the tabs.
	 * @default 'primary'
	 */
	variant?: TabVariants;
	/** The value of the tab that should be active when initially rendered. */
	defaultValue?: string;
	/** The controlled value of the tab to activate. */
	value?: string;
	/** Event handler called when the active tab changes. */
	onChange?: (key: string) => void;
	/** The orientation of the tabs. */
	orientation?: 'horizontal' | 'vertical';
	/** The direction of navigation when using keyboard. */
	dir?: 'ltr' | 'rtl';
	/**
	 * When automatic, tabs are activated when receiving focus.
	 * When manual, tabs are activated when clicked.
	 * @default 'automatic'
	 */
	activationMode?: 'automatic' | 'manual';
};

/**
 * Tabs component for organizing content into separate views.
 * Renders a full tabs set from an `items` array.
 */
export const Tabs = React.forwardRef<HTMLDivElement, TabsProps>(
	({ items, onChange, defaultValue, value, variant = 'primary', className, testId, ...props }, ref) => (
		<TabsRoot
			ref={ref}
			onValueChange={onChange}
			defaultValue={defaultValue ?? items[0]?.key}
			value={value}
			className={className}
			testId={testId}
			{...props}
		>
			<TabsList variant={variant}>
				{items.map((item) => (
					<TabsTrigger
						key={item.key}
						value={item.key}
						disabled={item.disabled}
						variant={variant}
						title={item.disabled ? item.disabledReason || 'This tab is disabled' : undefined}
					>
						{item.disabled ? (
							<Lock data-slot="tabs-icon" className="tabs-icon" size={16} />
						) : (
							item.prefixIcon && (
								<span data-slot="tabs-icon" className="tabs-icon">
									{item.prefixIcon}
								</span>
							)
						)}
						{item.label}
						{!item.disabled && item.suffixIcon && (
							<span data-slot="tabs-icon" className="tabs-icon">
								{item.suffixIcon}
							</span>
						)}
					</TabsTrigger>
				))}
			</TabsList>
			{items.map((item) => (
				<TabsContent key={item.key} value={item.key}>
					{item.children}
				</TabsContent>
			))}
		</TabsRoot>
	)
);
Tabs.displayName = 'Tabs';
