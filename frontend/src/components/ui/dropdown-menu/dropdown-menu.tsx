import './index.css';
import * as DropdownMenuPrimitive from '@radix-ui/react-dropdown-menu';
import { Check, ChevronLeft, ChevronRight, LoaderCircle } from 'lucide-react';
import * as React from 'react';

import { cn } from '../lib/utils';

type OriginalContentProps = React.ComponentProps<typeof DropdownMenuPrimitive.Content>;
type OriginalSubContentProps = React.ComponentProps<typeof DropdownMenuPrimitive.SubContent>;

export type DropdownMenuProps = {
	children?: React.ReactNode;
	open?: boolean;
	defaultOpen?: boolean;
	onOpenChange?: (open: boolean) => void;
	/** @default true */
	modal?: boolean;
	dir?: 'ltr' | 'rtl';
};

/**
 * Root component that manages the open state and accessibility wiring for a dropdown menu.
 */
function DropdownMenu(props: DropdownMenuProps) {
	return <DropdownMenuPrimitive.Root data-slot="dropdown-menu" {...props} />;
}

export type DropdownMenuPortalProps = React.ComponentProps<typeof DropdownMenuPrimitive.Portal>;

/**
 * Portals the dropdown menu content into `document.body`. Used internally by `DropdownMenuContent`.
 */
function DropdownMenuPortal(props: DropdownMenuPortalProps) {
	return <DropdownMenuPrimitive.Portal data-slot="dropdown-menu-portal" {...props} />;
}

export type DropdownMenuTriggerProps = React.ComponentProps<
	typeof DropdownMenuPrimitive.Trigger
> & {
	testId?: string;
};

/**
 * The button that toggles the dropdown menu. Use `asChild` to delegate to a child element.
 */
const DropdownMenuTrigger = React.forwardRef<HTMLButtonElement, DropdownMenuTriggerProps>(
	({ testId, ...props }, ref) => (
		<DropdownMenuPrimitive.Trigger
			ref={ref}
			data-slot="dropdown-menu-trigger"
			data-testid={testId}
			{...props}
		/>
	)
);
DropdownMenuTrigger.displayName = 'DropdownMenuTrigger';

export type DropdownMenuContentProps = {
	children?: React.ReactNode;
	className?: string;
	style?: OriginalContentProps['style'];
	id?: string;
	testId?: string;
	forceMount?: true;
	/** @default false */
	loop?: boolean;
	onCloseAutoFocus?: OriginalContentProps['onCloseAutoFocus'];
	onOpenAutoFocus?: (event: Event) => void;
	onEscapeKeyDown?: (event: KeyboardEvent) => void;
	onPointerDownOutside?: OriginalContentProps['onPointerDownOutside'];
	onFocusOutside?: OriginalContentProps['onFocusOutside'];
	onInteractOutside?: OriginalContentProps['onInteractOutside'];
	/** @default "bottom" */
	side?: 'top' | 'right' | 'bottom' | 'left';
	/** @default 4 */
	sideOffset?: number;
	/** @default "center" */
	align?: 'start' | 'center' | 'end';
	/** @default 0 */
	alignOffset?: number;
	/** @default true */
	avoidCollisions?: boolean;
	collisionBoundary?: OriginalContentProps['collisionBoundary'];
	collisionPadding?: OriginalContentProps['collisionPadding'];
	/** @default 0 */
	arrowPadding?: number;
	/** @default "partial" */
	sticky?: 'partial' | 'always';
	/** @default false */
	hideWhenDetached?: boolean;
	onKeyDown?: React.KeyboardEventHandler<HTMLDivElement>;
	onClick?: React.MouseEventHandler<HTMLDivElement>;
};

/**
 * The content that pops out when the dropdown menu is open. Rendered in a portal by default.
 */
const DropdownMenuContent = React.forwardRef<HTMLDivElement, DropdownMenuContentProps>(
	({ className, sideOffset = 4, testId, id, onClick, ...props }, ref) => (
		<DropdownMenuPortal>
			<DropdownMenuPrimitive.Content
				ref={ref}
				data-slot="dropdown-menu-content"
				data-testid={testId}
				id={id}
				sideOffset={sideOffset}
				className={cn(className)}
				onClick={(event) => {
					onClick?.(event);
					event.stopPropagation();
				}}
				{...props}
			/>
		</DropdownMenuPortal>
	)
);
DropdownMenuContent.displayName = 'DropdownMenuContent';

export type DropdownMenuGroupProps = React.ComponentProps<typeof DropdownMenuPrimitive.Group>;

/**
 * Groups related menu items together. Use with `DropdownMenuLabel` for a heading.
 */
const DropdownMenuGroup = React.forwardRef<HTMLDivElement, DropdownMenuGroupProps>(
	(props, ref) => (
		<DropdownMenuPrimitive.Group ref={ref} data-slot="dropdown-menu-group" {...props} />
	)
);
DropdownMenuGroup.displayName = 'DropdownMenuGroup';

export type DropdownMenuItemProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.Item>,
	'asChild'
> & {
	className?: string;
	testId?: string;
	/** When `true`, adds additional left padding. */
	inset?: boolean;
	/** Optional icon to display before the label. */
	leftIcon?: React.ReactNode;
	/** Optional icon to display after the label. */
	rightIcon?: React.ReactNode;
	/** When `true`, the item will be styled as destructive. */
	destructive?: boolean;
	/** When `true`, renders the item with `cursor: pointer`. */
	clickable?: boolean;
	disabled?: boolean;
	onSelect?: (event: Event) => void;
	textValue?: string;
};

/**
 * A selectable item in the dropdown menu.
 */
const DropdownMenuItem = React.forwardRef<HTMLDivElement, DropdownMenuItemProps>(
	({ className, inset, leftIcon, rightIcon, destructive, clickable, children, testId, ...props }, ref) => (
		<DropdownMenuPrimitive.Item
			ref={ref}
			data-slot="dropdown-menu-item"
			data-inset={inset ? '' : undefined}
			data-destructive={destructive ? '' : undefined}
			data-clickable={clickable ? '' : undefined}
			data-testid={testId}
			className={cn(className)}
			{...props}
		>
			{leftIcon && <span data-slot="dropdown-menu-item-icon">{leftIcon}</span>}
			{children}
			{rightIcon && <span data-slot="dropdown-menu-item-right-icon">{rightIcon}</span>}
		</DropdownMenuPrimitive.Item>
	)
);
DropdownMenuItem.displayName = 'DropdownMenuItem';

export type DropdownMenuCheckboxItemProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.CheckboxItem>,
	'asChild'
> & {
	className?: string;
	checked?: boolean | 'indeterminate';
	onCheckedChange?: (checked: boolean) => void;
	disabled?: boolean;
	onSelect?: (event: Event) => void;
	textValue?: string;
};

/**
 * A checkbox item in the dropdown menu that can be checked or unchecked.
 */
const DropdownMenuCheckboxItem = React.forwardRef<HTMLDivElement, DropdownMenuCheckboxItemProps>(
	({ className, children, checked, onSelect, ...props }, ref) => {
		const handleSelect = React.useCallback(
			(event: Event) => {
				event.preventDefault();
				onSelect?.(event);
			},
			[onSelect]
		);
		return (
			<DropdownMenuPrimitive.CheckboxItem
				ref={ref}
				data-slot="dropdown-menu-checkbox-item"
				className={cn(className)}
				checked={checked}
				onSelect={handleSelect}
				{...props}
			>
				<span data-slot="dropdown-menu-checkbox-indicator">
					<DropdownMenuPrimitive.ItemIndicator>
						<Check size={14} />
					</DropdownMenuPrimitive.ItemIndicator>
				</span>
				{children}
			</DropdownMenuPrimitive.CheckboxItem>
		);
	}
);
DropdownMenuCheckboxItem.displayName = 'DropdownMenuCheckboxItem';

export type DropdownMenuRadioGroupProps = React.ComponentProps<
	typeof DropdownMenuPrimitive.RadioGroup
>;

/**
 * Groups multiple `DropdownMenuRadioItem` components together.
 */
const DropdownMenuRadioGroup = React.forwardRef<HTMLDivElement, DropdownMenuRadioGroupProps>(
	(props, ref) => (
		<DropdownMenuPrimitive.RadioGroup
			ref={ref}
			data-slot="dropdown-menu-radio-group"
			{...props}
		/>
	)
);
DropdownMenuRadioGroup.displayName = 'DropdownMenuRadioGroup';

export type DropdownMenuRadioItemProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.RadioItem>,
	'asChild'
> & {
	className?: string;
	value: string;
	disabled?: boolean;
	onSelect?: (event: Event) => void;
	textValue?: string;
};

/**
 * A radio item in the dropdown menu. Must be used inside `DropdownMenuRadioGroup`.
 */
const DropdownMenuRadioItem = React.forwardRef<HTMLDivElement, DropdownMenuRadioItemProps>(
	({ className, children, ...props }, ref) => (
		<DropdownMenuPrimitive.RadioItem
			ref={ref}
			data-slot="dropdown-menu-radio-item"
			className={cn(className)}
			{...props}
		>
			<span data-slot="dropdown-menu-radio-indicator">
				<DropdownMenuPrimitive.ItemIndicator>
					<Check size={14} />
				</DropdownMenuPrimitive.ItemIndicator>
			</span>
			{children}
		</DropdownMenuPrimitive.RadioItem>
	)
);
DropdownMenuRadioItem.displayName = 'DropdownMenuRadioItem';

export type DropdownMenuLabelProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.Label>,
	'asChild'
> & {
	className?: string;
	/** When `true`, adds additional left padding. */
	inset?: boolean;
};

/**
 * A label for a group of items.
 */
const DropdownMenuLabel = React.forwardRef<HTMLDivElement, DropdownMenuLabelProps>(
	({ className, inset, ...props }, ref) => (
		<DropdownMenuPrimitive.Label
			ref={ref}
			data-slot="dropdown-menu-label"
			data-inset={inset ? '' : undefined}
			className={cn(className)}
			{...props}
		/>
	)
);
DropdownMenuLabel.displayName = 'DropdownMenuLabel';

export type DropdownMenuSeparatorProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.Separator>,
	'asChild'
> & {
	className?: string;
};

/**
 * A visual divider between sections in the dropdown menu.
 */
const DropdownMenuSeparator = React.forwardRef<HTMLDivElement, DropdownMenuSeparatorProps>(
	({ className, ...props }, ref) => (
		<DropdownMenuPrimitive.Separator
			ref={ref}
			data-slot="dropdown-menu-separator"
			className={cn(className)}
			{...props}
		/>
	)
);
DropdownMenuSeparator.displayName = 'DropdownMenuSeparator';

export type DropdownMenuShortcutProps = React.HTMLAttributes<HTMLSpanElement> & {
	className?: string;
};

/**
 * Right-aligned helper text, typically used to display keyboard shortcuts.
 */
const DropdownMenuShortcut = React.forwardRef<HTMLSpanElement, DropdownMenuShortcutProps>(
	({ className, ...props }, ref) => (
		<span ref={ref} data-slot="dropdown-menu-shortcut" className={cn(className)} {...props} />
	)
);
DropdownMenuShortcut.displayName = 'DropdownMenuShortcut';

export type DropdownMenuSubProps = {
	children?: React.ReactNode;
	open?: boolean;
	defaultOpen?: boolean;
	onOpenChange?: (open: boolean) => void;
};

/**
 * Contains all the parts of a submenu.
 */
function DropdownMenuSub(props: DropdownMenuSubProps) {
	return <DropdownMenuPrimitive.Sub data-slot="dropdown-menu-sub" {...props} />;
}

export type DropdownMenuSubContentProps = {
	children?: React.ReactNode;
	className?: string;
	forceMount?: true;
	/** @default false */
	loop?: boolean;
	onEscapeKeyDown?: (event: KeyboardEvent) => void;
	onPointerDownOutside?: OriginalSubContentProps['onPointerDownOutside'];
	onFocusOutside?: OriginalSubContentProps['onFocusOutside'];
	onInteractOutside?: OriginalSubContentProps['onInteractOutside'];
	/** @default 0 */
	sideOffset?: number;
	/** @default 0 */
	alignOffset?: number;
	/** @default true */
	avoidCollisions?: boolean;
	collisionBoundary?: OriginalSubContentProps['collisionBoundary'];
	collisionPadding?: OriginalSubContentProps['collisionPadding'];
	/** @default 0 */
	arrowPadding?: number;
	/** @default "partial" */
	sticky?: 'partial' | 'always';
	/** @default false */
	hideWhenDetached?: boolean;
};

/**
 * The content that pops out when a submenu is open. Must be rendered inside `DropdownMenuSub`.
 */
const DropdownMenuSubContent = React.forwardRef<HTMLDivElement, DropdownMenuSubContentProps>(
	({ className, ...props }, ref) => (
		<DropdownMenuPortal>
			<DropdownMenuPrimitive.SubContent
				ref={ref}
				data-slot="dropdown-menu-sub-content"
				className={cn(className)}
				{...props}
			/>
		</DropdownMenuPortal>
	)
);
DropdownMenuSubContent.displayName = 'DropdownMenuSubContent';

export type DropdownMenuSubTriggerProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.SubTrigger>,
	'asChild'
> & {
	className?: string;
	/** When `true`, adds additional left padding. */
	inset?: boolean;
	/** Optional icon to display before the label. */
	leftIcon?: React.ReactNode;
	disabled?: boolean;
	textValue?: string;
};

/**
 * An item that opens a submenu. Must be rendered inside `DropdownMenuSub`.
 */
const DropdownMenuSubTrigger = React.forwardRef<HTMLDivElement, DropdownMenuSubTriggerProps>(
	({ className, inset, leftIcon, children, ...props }, ref) => (
		<DropdownMenuPrimitive.SubTrigger
			ref={ref}
			data-slot="dropdown-menu-sub-trigger"
			data-inset={inset ? '' : undefined}
			className={cn(className)}
			{...props}
		>
			{leftIcon && <span data-slot="dropdown-menu-sub-trigger-icon">{leftIcon}</span>}
			{children}
			<ChevronRight data-slot="dropdown-menu-sub-trigger-chevron" />
		</DropdownMenuPrimitive.SubTrigger>
	)
);
DropdownMenuSubTrigger.displayName = 'DropdownMenuSubTrigger';

export type DropdownMenuLoadingProps = {
	className?: string;
	/** @default "Loading..." */
	text?: string;
};

/**
 * A loading state indicator for the dropdown menu.
 */
const DropdownMenuLoading = React.forwardRef<HTMLDivElement, DropdownMenuLoadingProps>(
	({ className, text = 'Loading...' }, ref) => (
		<div ref={ref} data-slot="dropdown-menu-loading" className={cn(className)}>
			<LoaderCircle data-slot="dropdown-menu-loading-spinner" />
			<span>{text}</span>
		</div>
	)
);
DropdownMenuLoading.displayName = 'DropdownMenuLoading';

export type DropdownMenuBackProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.Item>,
	'asChild' | 'onSelect'
> & {
	className?: string;
	/** The label to display next to the back icon. */
	label: string;
	/** Callback fired when the back button is clicked. */
	onBack?: () => void;
};

/**
 * A back button for navigating in multi-step dropdown menus.
 */
const DropdownMenuBack = React.forwardRef<HTMLDivElement, DropdownMenuBackProps>(
	({ className, label, onBack, ...props }, ref) => (
		<DropdownMenuPrimitive.Item
			ref={ref}
			data-slot="dropdown-menu-back"
			className={cn(className)}
			onSelect={(e) => {
				e.preventDefault();
				onBack?.();
			}}
			{...props}
		>
			<ChevronLeft data-slot="dropdown-menu-back-icon" />
			<span>{label}</span>
		</DropdownMenuPrimitive.Item>
	)
);
DropdownMenuBack.displayName = 'DropdownMenuBack';

export {
	DropdownMenu,
	DropdownMenuPortal,
	DropdownMenuTrigger,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuCheckboxItem,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuShortcut,
	DropdownMenuSub,
	DropdownMenuSubContent,
	DropdownMenuSubTrigger,
	DropdownMenuLoading,
	DropdownMenuBack,
};
