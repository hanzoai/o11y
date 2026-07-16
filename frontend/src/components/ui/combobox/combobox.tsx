import * as PopoverPrimitive from '@radix-ui/react-popover';
import {
	Command,
	CommandEmpty,
	CommandGroup,
	CommandInput,
	CommandItem,
	CommandList,
	CommandLoading,
	CommandSeparator,
} from 'cmdk';
import { Check, ChevronDown, Search, X } from 'lucide-react';
import {
	Children,
	type ComponentPropsWithoutRef,
	type CSSProperties,
	type FC,
	forwardRef,
	type KeyboardEvent,
	type ReactNode,
	useRef,
} from 'react';
import { cn } from '../lib/utils';

const itemBaseClass =
	'relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-[13px] leading-5 outline-none data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 data-[selected=true]:bg-[var(--accent)] data-[selected=true]:text-[var(--accent-foreground)]';

/**
 * Root component for the combobox. Controls open/close state of the popover.
 */
const Combobox: FC<PopoverPrimitive.PopoverProps> = PopoverPrimitive.Root;

export type ComboboxTriggerProps = Omit<
	ComponentPropsWithoutRef<typeof PopoverPrimitive.Trigger>,
	'value' | 'id' | 'className'
> & {
	id?: string;
	className?: string;
	testId?: string;
	placeholder?: ReactNode;
	value?: ReactNode;
	/**
	 * When true, renders child element as trigger instead of default button.
	 */
	asChild?: boolean;
};

/**
 * Trigger button that opens the combobox popover and displays the selected value.
 */
const ComboboxTrigger = forwardRef<HTMLButtonElement, ComboboxTriggerProps>(
	(
		{ className, placeholder, value, testId, id, asChild, children, ...props },
		ref,
	) => {
		if (asChild) {
			return (
				<PopoverPrimitive.Trigger ref={ref} asChild {...props}>
					{children}
				</PopoverPrimitive.Trigger>
			);
		}
		return (
			<PopoverPrimitive.Trigger
				ref={ref}
				id={id}
				data-slot="combobox-trigger"
				data-testid={testId}
				className={cn(
					'flex h-8 w-full items-center justify-between gap-2 whitespace-nowrap rounded-[calc(var(--radius)-2px)] border border-[var(--input)] bg-transparent px-3 py-2 text-[13px] leading-5 shadow-sm cursor-pointer focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--ring)] disabled:cursor-not-allowed disabled:opacity-50',
					className,
				)}
				{...props}
			>
				<span
					data-slot="combobox-value"
					className="flex items-center gap-2 overflow-hidden text-[13px] leading-5 [&_svg]:size-4 [&_svg]:shrink-0"
				>
					{value || placeholder || 'Select an option...'}
				</span>
				<ChevronDown
					data-slot="combobox-icon"
					className="size-4 shrink-0 opacity-50"
				/>
			</PopoverPrimitive.Trigger>
		);
	},
);
ComboboxTrigger.displayName = 'ComboboxTrigger';

export type ComboboxContentProps = ComponentPropsWithoutRef<
	typeof PopoverPrimitive.Content
> & {
	testId?: string;
	/**
	 * Only change to false when you want to include this component inside a popover.
	 * @default true
	 */
	withPortal?: boolean;
};

/**
 * Popover content container that wraps the combobox command and list.
 */
const ComboboxContent = forwardRef<HTMLDivElement, ComboboxContentProps>(
	(
		{
			className,
			testId,
			withPortal = true,
			align = 'start',
			sideOffset = 4,
			...props
		},
		ref,
	) => {
		const content = (
			<PopoverPrimitive.Content
				ref={ref}
				data-slot="combobox-content"
				data-testid={testId}
				align={align}
				sideOffset={sideOffset}
				className={cn(
					'z-50 min-w-[var(--radix-popover-trigger-width)] overflow-hidden rounded-[2px] border border-[var(--border)] bg-[var(--popover)] p-0 text-[var(--popover-foreground)] shadow-md',
					'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
					className,
				)}
				{...props}
			/>
		);
		if (!withPortal) {
			return content;
		}
		return <PopoverPrimitive.Portal>{content}</PopoverPrimitive.Portal>;
	},
);
ComboboxContent.displayName = 'ComboboxContent';

export type ComboboxCommandProps = ComponentPropsWithoutRef<typeof Command> & {
	testId?: string;
};

/**
 * Command root used inside the combobox for filtering and keyboard navigation.
 */
const ComboboxCommand = forwardRef<HTMLDivElement, ComboboxCommandProps>(
	({ className, testId, ...props }, ref) => (
		<Command
			ref={ref}
			data-slot="combobox-command"
			data-testid={testId}
			className={cn('flex h-full w-full flex-col overflow-hidden', className)}
			{...props}
		/>
	),
);
ComboboxCommand.displayName = 'ComboboxCommand';

export type ComboboxInputProps = ComponentPropsWithoutRef<
	typeof CommandInput
> & {
	testId?: string;
	containerClassName?: string;
	containerStyle?: CSSProperties;
	containerId?: string;
	containerTestId?: string;
};

/**
 * Search input inside the combobox.
 */
const ComboboxInput = forwardRef<HTMLInputElement, ComboboxInputProps>(
	(
		{
			className,
			testId,
			containerClassName,
			containerStyle,
			containerId,
			containerTestId,
			...props
		},
		ref,
	) => (
		<div
			data-slot="combobox-input-wrapper"
			id={containerId}
			data-testid={containerTestId}
			style={containerStyle}
			className={cn(
				'flex items-center gap-2 border-b border-[var(--border)] px-3',
				containerClassName,
			)}
		>
			<Search className="size-4 shrink-0 opacity-50" />
			<CommandInput
				ref={ref}
				data-slot="combobox-input"
				data-testid={testId}
				className={cn(
					'flex h-9 w-full bg-transparent py-2 text-[13px] leading-5 outline-none placeholder:text-[var(--muted-foreground)] disabled:cursor-not-allowed disabled:opacity-50',
					className,
				)}
				{...props}
			/>
		</div>
	),
);
ComboboxInput.displayName = 'ComboboxInput';

export type ComboboxListProps = ComponentPropsWithoutRef<typeof CommandList>;

/**
 * Scrollable list container for combobox items.
 */
const ComboboxList = forwardRef<HTMLDivElement, ComboboxListProps>(
	({ className, ...props }, ref) => (
		<CommandList
			ref={ref}
			data-slot="combobox-list"
			className={cn(
				'max-h-[300px] overflow-y-auto overflow-x-hidden p-1',
				className,
			)}
			{...props}
		/>
	),
);
ComboboxList.displayName = 'ComboboxList';

export type ComboboxEmptyProps = ComponentPropsWithoutRef<typeof CommandEmpty>;

/**
 * Fallback content shown when there are no matching results.
 */
const ComboboxEmpty = forwardRef<HTMLDivElement, ComboboxEmptyProps>(
	({ className, ...props }, ref) => (
		<CommandEmpty
			ref={ref}
			data-slot="combobox-empty"
			className={cn(
				'py-6 text-center text-[13px] text-[var(--muted-foreground)]',
				className,
			)}
			{...props}
		/>
	),
);
ComboboxEmpty.displayName = 'ComboboxEmpty';

export type ComboboxLoadingProps = ComponentPropsWithoutRef<
	typeof CommandLoading
> & {
	testId?: string;
};

/**
 * Loading indicator shown while fetching or filtering items.
 */
const ComboboxLoading = forwardRef<HTMLDivElement, ComboboxLoadingProps>(
	({ className, testId, ...props }, ref) => (
		<CommandLoading
			ref={ref}
			data-slot="combobox-loading"
			data-testid={testId}
			className={cn(
				'flex items-center justify-center gap-2 px-2 py-3 text-[13px] text-[var(--muted-foreground)]',
				className,
			)}
			{...props}
		/>
	),
);
ComboboxLoading.displayName = 'ComboboxLoading';

export type ComboboxGroupProps = ComponentPropsWithoutRef<
	typeof CommandGroup
> & {
	testId?: string;
};

/**
 * Groups related combobox items.
 */
const ComboboxGroup = forwardRef<HTMLDivElement, ComboboxGroupProps>(
	({ className, testId, children, ...props }, ref) => (
		<CommandGroup
			ref={ref}
			data-slot="combobox-group"
			data-testid={testId}
			className={cn(
				'overflow-hidden text-[var(--popover-foreground)] [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-[var(--muted-foreground)]',
				className,
			)}
			{...props}
		>
			{children}
		</CommandGroup>
	),
);
ComboboxGroup.displayName = 'ComboboxGroup';

export type ComboboxItemProps = ComponentPropsWithoutRef<typeof CommandItem> & {
	isSelected?: boolean;
	/**
	 * When true, inserts value into input instead of selecting it.
	 * @default false
	 */
	insertOnInput?: boolean;
	/**
	 * Callback when item is used to insert into input. Called with the value to insert.
	 */
	onInsert?: (value: string) => void;
	prefix?: ReactNode | null;
	suffix?: ReactNode | null;
	testId?: string;
};

/**
 * Selectable item in the combobox list.
 */
const ComboboxItem = forwardRef<HTMLDivElement, ComboboxItemProps>(
	(
		{
			className,
			prefix,
			suffix,
			isSelected = false,
			insertOnInput = false,
			onInsert,
			onSelect,
			value,
			testId,
			children,
			...props
		},
		ref,
	) => {
		const resolvedPrefix =
			prefix === undefined ? (
				<span
					data-slot="combobox-item-indicator"
					data-selected={isSelected}
					className="flex size-4 shrink-0 items-center justify-center opacity-0 data-[selected=true]:opacity-100 [&_svg]:size-4"
				>
					<Check />
				</span>
			) : (
				prefix
			);

		const handleSelect = (currentValue: string): void => {
			if (insertOnInput && onInsert) {
				onInsert(value ?? currentValue);
			} else {
				onSelect?.(currentValue);
			}
		};

		return (
			<CommandItem
				ref={ref}
				data-slot="combobox-item"
				data-insert-on-input={insertOnInput}
				data-testid={testId}
				value={value}
				onSelect={handleSelect}
				className={cn(itemBaseClass, className)}
				{...props}
			>
				{resolvedPrefix != null && (
					<span
						data-slot="combobox-item-prefix"
						className="flex shrink-0 items-center"
					>
						{resolvedPrefix}
					</span>
				)}
				{children}
				{suffix != null && (
					<span
						data-slot="combobox-item-suffix"
						className="ml-auto flex shrink-0 items-center"
					>
						{suffix}
					</span>
				)}
			</CommandItem>
		);
	},
);
ComboboxItem.displayName = 'ComboboxItem';

export type ComboboxSeparatorProps = ComponentPropsWithoutRef<
	typeof CommandSeparator
>;

/**
 * Visual divider between groups inside the combobox list.
 */
const ComboboxSeparator = forwardRef<HTMLDivElement, ComboboxSeparatorProps>(
	({ className, ...props }, ref) => (
		<CommandSeparator
			ref={ref}
			data-slot="combobox-separator"
			className={cn('-mx-1 my-1 h-px bg-[var(--border)]', className)}
			{...props}
		/>
	),
);
ComboboxSeparator.displayName = 'ComboboxSeparator';

export type ComboboxPillProps = {
	/** The value represented by this pill. */
	value: string;
	/** Callback fired when the remove button is clicked. */
	onRemove: (value: string) => void;
	/** Content to render inside the pill. */
	children: ReactNode;
	/** Additional CSS class names. */
	className?: string;
};

/**
 * Removable pill/tag for multi-select combobox.
 */
const ComboboxPill = forwardRef<HTMLSpanElement, ComboboxPillProps>(
	({ value, onRemove, children, className }, ref) => (
		<span
			ref={ref}
			data-slot="combobox-pill"
			className={cn(
				'inline-flex h-5 items-center gap-0.5 rounded-[2px] bg-[var(--accent)] px-1.5 text-xs leading-none text-[var(--accent-foreground)]',
				className,
			)}
		>
			<span
				data-slot="combobox-pill-text"
				className="max-w-32 overflow-hidden text-ellipsis whitespace-nowrap"
			>
				{children}
			</span>
			<button
				type="button"
				data-slot="combobox-pill-remove"
				aria-label={`Remove ${value}`}
				className="flex size-3.5 items-center justify-center rounded-[2px] opacity-70 transition-opacity hover:bg-black/10 hover:opacity-100 [&_svg]:size-2.5"
				onClick={(e): void => {
					e.preventDefault();
					e.stopPropagation();
					onRemove(value);
				}}
			>
				<X />
			</button>
		</span>
	),
);
ComboboxPill.displayName = 'ComboboxPill';

export type ComboboxMultiTriggerProps = {
	className?: string;
	style?: CSSProperties;
	id?: string;
	testId?: string;
	placeholder?: string;
	inputValue: string;
	onInputChange: (value: string) => void;
	onKeyDown?: (e: KeyboardEvent<HTMLInputElement>) => void;
	onFocus?: () => void;
	disabled?: boolean;
	children?: ReactNode;
};

/**
 * Multi-select trigger with inline input and pills.
 */
const ComboboxMultiTrigger = forwardRef<
	HTMLDivElement,
	ComboboxMultiTriggerProps
>(
	(
		{
			className,
			style,
			id,
			testId,
			placeholder,
			inputValue,
			onInputChange,
			onKeyDown,
			onFocus,
			disabled,
			children,
		},
		ref,
	) => {
		const inputRef = useRef<HTMLInputElement>(null);
		const handleContainerClick = (): void => {
			if (!disabled) {
				inputRef.current?.focus();
			}
		};
		return (
			// The container is a click target that forwards focus to the inner text input.
			// eslint-disable-next-line jsx-a11y/no-static-element-interactions, jsx-a11y/click-events-have-key-events
			<div
				ref={ref}
				id={id}
				data-slot="combobox-multi-trigger"
				data-testid={testId}
				data-disabled={disabled}
				style={style}
				className={cn(
					'flex min-h-8 w-full flex-wrap items-center gap-1 rounded-[calc(var(--radius)-2px)] border border-[var(--input)] bg-transparent px-2 py-1 text-[13px] leading-5 shadow-sm cursor-text focus-within:outline-none focus-within:ring-1 focus-within:ring-[var(--ring)] data-[disabled=true]:cursor-not-allowed data-[disabled=true]:opacity-50',
					className,
				)}
				onClick={handleContainerClick}
			>
				{children}
				<input
					ref={inputRef}
					type="text"
					data-slot="combobox-multi-input"
					className="min-w-16 flex-1 border-0 bg-transparent py-0.5 text-[13px] leading-5 outline-none placeholder:text-[var(--muted-foreground)]"
					placeholder={Children.count(children) === 0 ? placeholder : undefined}
					value={inputValue}
					onChange={(e): void => onInputChange(e.target.value)}
					onKeyDown={onKeyDown}
					onFocus={onFocus}
					disabled={disabled}
				/>
			</div>
		);
	},
);
ComboboxMultiTrigger.displayName = 'ComboboxMultiTrigger';

export type ComboboxCreateItemProps = Omit<ComboboxItemProps, 'children'> & {
	/** The input value to create. */
	inputValue: string;
	/** Custom render function for the create option. */
	children?: ReactNode;
};

/**
 * Option to create a new item from the current input value.
 */
const ComboboxCreateItem = forwardRef<HTMLDivElement, ComboboxCreateItemProps>(
	({ inputValue, children, className, prefix = null, ...props }, ref) => (
		<CommandItem
			ref={ref}
			data-slot="combobox-create-item"
			className={cn(
				itemBaseClass,
				'italic text-[var(--muted-foreground)]',
				className,
			)}
			{...props}
		>
			{prefix != null && (
				<span className="flex shrink-0 items-center">{prefix}</span>
			)}
			{children ?? `Create "${inputValue}"`}
		</CommandItem>
	),
);
ComboboxCreateItem.displayName = 'ComboboxCreateItem';

export type ComboboxHintProps = Omit<ComboboxItemProps, 'onSelect'> & {
	/** The value to insert into the input when this hint is selected. */
	insertValue: string;
	/** Callback when hint is selected. Called with the insertValue. */
	onInsert: (value: string) => void;
	/** Custom content for the hint. */
	children: ReactNode;
};

/**
 * Hint item that inserts a value into the input instead of selecting it.
 */
const ComboboxHint = forwardRef<HTMLDivElement, ComboboxHintProps>(
	(
		{ insertValue, onInsert, children, className, prefix = null, ...props },
		ref,
	) => (
		<CommandItem
			ref={ref}
			data-slot="combobox-hint"
			className={cn(
				itemBaseClass,
				'italic text-[var(--muted-foreground)]',
				className,
			)}
			onSelect={(): void => onInsert(insertValue)}
			{...props}
		>
			{prefix != null && (
				<span className="flex shrink-0 items-center">{prefix}</span>
			)}
			{children}
		</CommandItem>
	),
);
ComboboxHint.displayName = 'ComboboxHint';

export {
	Combobox,
	ComboboxCommand,
	ComboboxContent,
	ComboboxCreateItem,
	ComboboxEmpty,
	ComboboxGroup,
	ComboboxHint,
	ComboboxInput,
	ComboboxItem,
	ComboboxList,
	ComboboxLoading,
	ComboboxMultiTrigger,
	ComboboxPill,
	ComboboxSeparator,
	ComboboxTrigger,
};
