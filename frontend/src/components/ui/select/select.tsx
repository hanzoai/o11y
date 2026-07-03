import './index.css';
import * as React from 'react';
import * as SelectPrimitive from '@radix-ui/react-select';
import { Check, ChevronDown, LoaderCircle, X } from 'lucide-react';
import { cn } from '../lib/utils';

/* -------------------------------------------------------------------------- */
/* Context                                                                    */
/* -------------------------------------------------------------------------- */

type SelectContextValue = {
	multiple: boolean;
	value: string[];
	onValueChange: (value: string) => void;
	onRemove: (value: string) => void;
};

const SelectContext = React.createContext<SelectContextValue | null>(null);

function useSelectContext(): SelectContextValue | null {
	return React.useContext(SelectContext);
}

/* -------------------------------------------------------------------------- */
/* Root                                                                       */
/* -------------------------------------------------------------------------- */

export type SelectProps = {
	/** The content of the select (trigger, content, items, etc.) */
	children: React.ReactNode;
	/** The controlled value of the select. Use with `onChange`. */
	value?: string | string[];
	/** The default value when uncontrolled. */
	defaultValue?: string | string[];
	/** Callback fired when the value changes. */
	onChange?: (value: string | string[]) => void;
	/** Whether multiple items can be selected. */
	multiple?: boolean;
	/** The controlled open state of the select. */
	open?: boolean;
	/** The default open state when uncontrolled. */
	defaultOpen?: boolean;
	/** Callback fired when the open state changes. */
	onOpenChange?: (open: boolean) => void;
	/** Whether the select is disabled. */
	disabled?: boolean;
	/** Whether the select is required in a form. */
	required?: boolean;
	/** The name of the select for form submission. */
	name?: string;
};

/**
 * Root component for the select. Controls open/close state and selection.
 * Supports both single and multiple selection.
 */
export function Select({
	children,
	value: controlledValue,
	defaultValue,
	onChange,
	multiple = false,
	open,
	defaultOpen,
	onOpenChange,
	disabled,
	required,
	name,
}: SelectProps): React.ReactElement {
	const normalizeValue = (v: string | string[] | undefined): string[] => {
		if (v === undefined) return [];
		return Array.isArray(v) ? v : [v];
	};

	const [internalValue, setInternalValue] = React.useState<string[]>(() =>
		normalizeValue(defaultValue)
	);
	const isControlled = controlledValue !== undefined;
	const currentValue = isControlled ? normalizeValue(controlledValue) : internalValue;

	const [internalOpen, setInternalOpen] = React.useState(defaultOpen ?? false);
	const isOpenControlled = open !== undefined;
	const currentOpen = isOpenControlled ? open : internalOpen;

	const handleOpenChange = React.useCallback(
		(newOpen: boolean) => {
			if (!isOpenControlled) setInternalOpen(newOpen);
			onOpenChange?.(newOpen);
		},
		[isOpenControlled, onOpenChange]
	);

	const handleValueChange = React.useCallback(
		(selectedValue: string) => {
			if (multiple) {
				const newValue = currentValue.includes(selectedValue)
					? currentValue.filter((v) => v !== selectedValue)
					: [...currentValue, selectedValue];
				if (!isControlled) setInternalValue(newValue);
				onChange?.(newValue);
			} else {
				if (!isControlled) setInternalValue([selectedValue]);
				onChange?.(selectedValue);
				handleOpenChange(false);
			}
		},
		[multiple, currentValue, isControlled, onChange, handleOpenChange]
	);

	const handleRemove = React.useCallback(
		(valueToRemove: string) => {
			const newValue = currentValue.filter((v) => v !== valueToRemove);
			if (!isControlled) setInternalValue(newValue);
			onChange?.(multiple ? newValue : newValue[0] ?? '');
		},
		[currentValue, isControlled, onChange, multiple]
	);

	const contextValue = React.useMemo<SelectContextValue>(
		() => ({
			multiple,
			value: currentValue,
			onValueChange: handleValueChange,
			onRemove: handleRemove,
		}),
		[multiple, currentValue, handleValueChange, handleRemove]
	);

	if (multiple) {
		return (
			<SelectContext.Provider value={contextValue}>
				<SelectPrimitive.Root
					open={currentOpen}
					onOpenChange={handleOpenChange}
					disabled={disabled}
					required={required}
					name={name}
					value=""
					onValueChange={handleValueChange}
				>
					{children}
				</SelectPrimitive.Root>
			</SelectContext.Provider>
		);
	}

	return (
		<SelectContext.Provider value={contextValue}>
			<SelectPrimitive.Root
				value={currentValue[0] ?? ''}
				defaultValue={typeof defaultValue === 'string' ? defaultValue : undefined}
				onValueChange={handleValueChange}
				open={currentOpen}
				onOpenChange={handleOpenChange}
				disabled={disabled}
				required={required}
				name={name}
			>
				{children}
			</SelectPrimitive.Root>
		</SelectContext.Provider>
	);
}

/* -------------------------------------------------------------------------- */
/* Trigger + Value                                                            */
/* -------------------------------------------------------------------------- */

export type SelectTriggerProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
	/** Placeholder text when no value is selected. */
	placeholder?: React.ReactNode;
	children?: React.ReactNode;
	/** Custom render function for the selected value(s). */
	renderValue?: (values: string[]) => React.ReactNode;
	/** Resolves a value to its display label. */
	resolveLabel?: (value: string) => React.ReactNode;
	/** Maximum number of pills to display in multi-select mode. */
	maxDisplayedPills?: number;
	disabled?: boolean;
	'aria-label'?: string;
	'aria-labelledby'?: string;
	'aria-describedby'?: string;
	/** Show loading spinner instead of chevron icon. */
	loading?: boolean;
};

/**
 * Trigger button that opens the select dropdown.
 * In single-select mode, displays the selected value or placeholder.
 * In multi-select mode, displays removable pills for each selected value.
 */
export const SelectTrigger = React.forwardRef<HTMLButtonElement, SelectTriggerProps>(
	(
		{ className, style, id, testId, placeholder, children, renderValue, resolveLabel, maxDisplayedPills, loading = false, ...props },
		ref
	) => {
		const context = useSelectContext();
		const hasValue = context && context.value.length > 0;

		const renderContent = (): React.ReactNode => {
			if (children) return children;
			if (renderValue && context) return renderValue(context.value);
			if (context?.multiple && hasValue) {
				const values = context.value;
				const displayedValues =
					maxDisplayedPills !== undefined ? values.slice(0, maxDisplayedPills) : values;
				const overflowCount =
					maxDisplayedPills !== undefined ? Math.max(0, values.length - maxDisplayedPills) : 0;
				return (
					<span data-slot="select-pills" className="select-pills">
						{displayedValues.map((v) => (
							<span data-slot="select-pill" className="select-pill" key={v}>
								<span data-slot="select-pill-text" className="select-pill-text">
									{resolveLabel ? resolveLabel(v) : v}
								</span>
								<button
									type="button"
									data-slot="select-pill-remove"
									className="select-pill-remove"
									onPointerDown={(e): void => {
										e.preventDefault();
										e.stopPropagation();
										context.onRemove(v);
									}}
									aria-label={`Remove ${v}`}
								>
									<X />
								</button>
							</span>
						))}
						{overflowCount > 0 && (
							<span data-slot="select-pill-overflow" className="select-pill-overflow">
								+{overflowCount}
							</span>
						)}
					</span>
				);
			}
			if (resolveLabel && hasValue && context) {
				return <span>{resolveLabel(context.value[0])}</span>;
			}
			return <SelectPrimitive.Value placeholder={placeholder} />;
		};

		return (
			<SelectPrimitive.Trigger
				ref={ref}
				id={id}
				className={cn('select-trigger', className)}
				style={style}
				data-slot="select-trigger"
				data-testid={testId}
				{...props}
			>
				<span data-slot="select-trigger-value" className="select-trigger-value">
					{renderContent()}
				</span>
				{loading ? (
					<LoaderCircle data-slot="select-trigger-spinner" className="select-trigger-spinner" />
				) : (
					<SelectPrimitive.Icon asChild>
						<ChevronDown data-slot="select-trigger-icon" className="select-trigger-icon" />
					</SelectPrimitive.Icon>
				)}
			</SelectPrimitive.Trigger>
		);
	}
);
SelectTrigger.displayName = 'SelectTrigger';

export type SelectValueProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
	placeholder?: React.ReactNode;
	children?: React.ReactNode;
};

/**
 * Renders the selected value text in single-select mode.
 */
export const SelectValue = React.forwardRef<
	React.ElementRef<typeof SelectPrimitive.Value>,
	SelectValueProps
>(({ className, style, id, testId, ...props }, ref) => (
	<SelectPrimitive.Value
		ref={ref}
		id={id}
		className={className}
		style={style}
		data-slot="select-value"
		data-testid={testId}
		{...props}
	/>
));
SelectValue.displayName = 'SelectValue';

/* -------------------------------------------------------------------------- */
/* Content                                                                    */
/* -------------------------------------------------------------------------- */

export type SelectContentProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
	children?: React.ReactNode;
	/** @default true */
	withPortal?: boolean;
	/** @default true */
	withViewport?: boolean;
	/** @default "popper" */
	position?: 'item-aligned' | 'popper';
	/** @default "bottom" */
	side?: 'top' | 'right' | 'bottom' | 'left';
	/** @default 4 */
	sideOffset?: number;
	/** @default "start" */
	align?: 'start' | 'center' | 'end';
	alignOffset?: number;
	avoidCollisions?: boolean;
	onEscapeKeyDown?: (event: KeyboardEvent) => void;
	onPointerDownOutside?: (event: Event) => void;
};

/**
 * Dropdown content container that holds the selectable items.
 */
export const SelectContent = React.forwardRef<HTMLDivElement, SelectContentProps>(
	({ className, style, id, testId, children, withPortal = true, withViewport = true, position = 'popper', sideOffset = 4, ...props }, ref) => {
		const content = (
			<SelectPrimitive.Content
				ref={ref}
				id={id}
				className={cn('select-content', className)}
				style={style}
				data-slot="select-content"
				data-testid={testId}
				position={position}
				sideOffset={sideOffset}
				{...props}
			>
				{withViewport ? (
					<SelectPrimitive.Viewport data-slot="select-viewport" className="select-viewport">
						{children}
					</SelectPrimitive.Viewport>
				) : (
					children
				)}
			</SelectPrimitive.Content>
		);
		if (withPortal) return <SelectPrimitive.Portal>{content}</SelectPrimitive.Portal>;
		return content;
	}
);
SelectContent.displayName = 'SelectContent';

/* -------------------------------------------------------------------------- */
/* Item                                                                       */
/* -------------------------------------------------------------------------- */

export type SelectItemProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
	/** The value of the item (used for selection). */
	value: string;
	children?: React.ReactNode;
	disabled?: boolean;
	/** Text value for typeahead. By default uses trimmed text content. */
	textValue?: string;
	/** Additional CSS class names for the check indicator. */
	indicatorClassname?: string;
};

/**
 * Selectable item within the dropdown.
 */
export const SelectItem = React.forwardRef<HTMLDivElement, SelectItemProps>(
	({ className, style, id, testId, indicatorClassname, children, ...props }, ref) => {
		const context = useSelectContext();
		const isSelected = context?.value.includes(props.value) ?? false;
		const handlePointerUp = (e: React.PointerEvent): void => {
			if (context?.multiple && !props.disabled) {
				e.preventDefault();
				context.onValueChange(props.value);
			}
		};
		return (
			<SelectPrimitive.Item
				ref={ref}
				id={id}
				className={cn('select-item', className)}
				style={style}
				data-slot="select-item"
				data-testid={testId}
				data-selected={isSelected}
				data-multiple={context?.multiple || undefined}
				onPointerUp={handlePointerUp}
				{...props}
			>
				{context?.multiple && (
					<span
						data-slot="select-item-indicator"
						data-selected={isSelected}
						className={cn('select-item-indicator', indicatorClassname)}
					>
						<SelectPrimitive.ItemIndicator>
							<Check />
						</SelectPrimitive.ItemIndicator>
						{isSelected && !props.disabled && <Check />}
					</span>
				)}
				<SelectPrimitive.ItemText>
					<span data-slot="select-item-container" className="select-item-container">
						{children}
					</span>
				</SelectPrimitive.ItemText>
			</SelectPrimitive.Item>
		);
	}
);
SelectItem.displayName = 'SelectItem';

/* -------------------------------------------------------------------------- */
/* Group + Label + Separator                                                  */
/* -------------------------------------------------------------------------- */

export type SelectGroupProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
	children?: React.ReactNode;
};

/**
 * Groups related select items together.
 */
export const SelectGroup = React.forwardRef<HTMLDivElement, SelectGroupProps>(
	({ className, style, id, testId, ...props }, ref) => (
		<SelectPrimitive.Group
			ref={ref}
			id={id}
			className={cn('select-group', className)}
			style={style}
			data-slot="select-group"
			data-testid={testId}
			{...props}
		/>
	)
);
SelectGroup.displayName = 'SelectGroup';

export type SelectLabelProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
	children?: React.ReactNode;
};

/**
 * Label for a group of select items.
 */
export const SelectLabel = React.forwardRef<HTMLDivElement, SelectLabelProps>(
	({ className, style, id, testId, ...props }, ref) => (
		<SelectPrimitive.Label
			ref={ref}
			id={id}
			className={cn('select-label', className)}
			style={style}
			data-slot="select-label"
			data-testid={testId}
			{...props}
		/>
	)
);
SelectLabel.displayName = 'SelectLabel';

export type SelectSeparatorProps = {
	className?: string;
	style?: React.CSSProperties;
	id?: string;
	testId?: string;
};

/**
 * Visual separator between select items or groups.
 */
export const SelectSeparator = React.forwardRef<HTMLDivElement, SelectSeparatorProps>(
	({ className, style, id, testId, ...props }, ref) => (
		<SelectPrimitive.Separator
			ref={ref}
			id={id}
			className={cn('select-separator', className)}
			style={style}
			data-slot="select-separator"
			data-testid={testId}
			{...props}
		/>
	)
);
SelectSeparator.displayName = 'SelectSeparator';
