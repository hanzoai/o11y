import './index.css';
import React from 'react';
import { cn } from '@hanzo/ui/core';

import { focusFirstItem, rovingKeyDown } from '../lib/roving-focus';
import { useControllable } from '../lib/use-controllable';

interface RadioGroupContextValue {
	value: string | null;
	select: (value: string) => void;
	name?: string;
	required?: boolean;
	disabled?: boolean;
	dir: 'ltr' | 'rtl';
}

const RadioGroupContext = React.createContext<RadioGroupContextValue | null>(
	null,
);

export type RadioColorProps =
	| 'robin'
	| 'forest'
	| 'amber'
	| 'sienna'
	| 'cherry'
	| 'sakura'
	| 'aqua';

export type RadioGroupProps = Pick<
	React.ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/**
	 * The name of the group. Submitted with its owning form as part of a name/value pair.
	 */
	name?: string;
	/**
	 * When true, indicates that the user must check a radio item before the owning form can be submitted.
	 */
	required?: boolean;
	/**
	 * When true, prevents the user from interacting with radio items.
	 */
	disabled?: boolean;
	/**
	 * The reading direction of the radio group. If omitted, inherits globally from DirectionProvider or assumes LTR (left-to-right) reading mode.
	 */
	dir?: 'ltr' | 'rtl';
	/**
	 * The orientation of the component.
	 */
	orientation?: React.AriaAttributes['aria-orientation'];
	/**
	 * When true, keyboard navigation will loop from last item to first, and vice versa.
	 */
	loop?: boolean;
	/**
	 * The value of the radio item that should be checked when initially rendered. Use when you do not need to control the state of the radio items.
	 */
	defaultValue?: string;
	/**
	 * The controlled value of the radio item to check. Should be used in conjunction with onChange.
	 */
	value?: string | null;
	/**
	 * Event handler called when the value changes.
	 */
	onChange?: (value: string) => void;
	/**
	 * The testId associated with the radio group.
	 */
	testId?: string;
	/**
	 * The color of the radio group.
	 */
	color?: RadioColorProps;
};

export type RadioGroupItemProps = Pick<
	React.ComponentPropsWithoutRef<'button'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/**
	 * The value given as data when submitted with a name.
	 */
	value: string;
	/**
	 * When true, indicates that the user must check the radio item before the owning form can be submitted.
	 */
	required?: boolean;
	/**
	 * When true, prevents the user from interacting with the radio item.
	 */
	disabled?: boolean;
	/**
	 * The testId associated with the radio item.
	 */
	testId?: string;
	/**
	 * Additional CSS classes to apply to the radio item wrapper.
	 */
	containerClassName?: string;
	/**
	 * Inline styles to apply to the radio item wrapper.
	 */
	containerStyle?: React.CSSProperties;
	/**
	 * The id of the radio item wrapper.
	 */
	containerId?: string;
	/**
	 * The testId associated with the radio item wrapper.
	 */
	containerTestId?: string;
	/**
	 * The callback invoked when the value state of the radio item changes.
	 */
	onCheck?(): void;
};

export type RadioGroupLabelProps = Pick<
	React.ComponentPropsWithoutRef<'label'>,
	'id' | 'className' | 'children' | 'htmlFor'
>;

/**
 * RadioGroup component for managing a group of radio button options.
 */
const RadioGroup = React.forwardRef<HTMLDivElement, RadioGroupProps>(
	(
		{
			className,
			onChange,
			color = 'robin',
			testId,
			value,
			defaultValue,
			name,
			required,
			disabled,
			dir = 'ltr',
			orientation,
			// `loop` is radix vocabulary the CSS and call sites never read: navigation
			// here always wraps, which is what every call site already relied on.
			loop: _loop,
			...props
		},
		ref,
	) => {
		const [selected, select] = useControllable<string | null>(
			value,
			defaultValue ?? null,
			onChange as ((next: string | null) => void) | undefined,
		);
		const context = React.useMemo<RadioGroupContextValue>(
			() => ({
				value: selected,
				select: select as (next: string) => void,
				name,
				required,
				disabled,
				dir,
			}),
			[selected, select, name, required, disabled, dir],
		);

		return (
			<RadioGroupContext.Provider value={context}>
				<div
					ref={ref}
					role="radiogroup"
					dir={dir}
					aria-orientation={orientation}
					aria-required={required || undefined}
					data-slot="radio-group"
					data-color={color}
					data-testid={testId}
					data-roving-group=""
					className={cn(className)}
					tabIndex={selected === null ? 0 : -1}
					onFocus={focusFirstItem}
					{...props}
				/>
			</RadioGroupContext.Provider>
		);
	},
);
RadioGroup.displayName = 'RadioGroup';

const RadioGroupLabel = React.forwardRef<
	HTMLLabelElement,
	RadioGroupLabelProps & React.AriaAttributes
>(({ className, htmlFor, ...props }, ref) => (
	<label
		ref={ref}
		htmlFor={htmlFor}
		data-slot="radio-group-label"
		className={cn(className)}
		{...props}
	/>
));
RadioGroupLabel.displayName = 'RadioGroupLabel';

const RadioGroupItem = React.forwardRef<HTMLButtonElement, RadioGroupItemProps>(
	(
		{
			className,
			style,
			children,
			testId,
			containerClassName,
			containerStyle,
			containerId,
			containerTestId,
			// onCheck is a documented-but-unwired Periscope prop; strip it so it never
			// reaches the DOM. It is a React callback prop, not a class method.
			// eslint-disable-next-line typescript-eslint/unbound-method
			onCheck: _onCheck,
			...props
		},
		ref,
	) => {
		const group = React.useContext(RadioGroupContext);
		const fallbackId = React.useId();
		const radioId = props.id || fallbackId;
		// `required` is the GROUP's fact (see the root's aria-required), so it is
		// destructured off here only to keep it off the DOM node.
		const { value, disabled, required: _required, ...rest } = props;
		const checked = group?.value === value;
		const isDisabled = disabled || group?.disabled;

		const item = (
			<button
				ref={ref}
				type="button"
				role="radio"
				aria-checked={checked}
				// aria-required belongs on the radiogroup, not on a radio — the
				// requirement is "pick one of these", not "pick this one". The root
				// carries it.
				data-slot="radio-group-item"
				data-state={checked ? 'checked' : 'unchecked'}
				data-roving-item=""
				data-testid={testId}
				className={cn(className)}
				style={style}
				disabled={isDisabled}
				name={group?.name}
				value={value}
				tabIndex={checked ? 0 : -1}
				onClick={(): void => group?.select(value)}
				onKeyDown={(event): void =>
					rovingKeyDown(
						event,
						(el) => el.click(),
						group?.dir ?? 'ltr',
					)
				}
				{...rest}
			>
				{checked && <span data-slot="radio-group-indicator" />}
			</button>
		);

		if (!children) return item;

		return (
			<div
				data-slot="radio-group-item-wrapper"
				className={cn(containerClassName)}
				data-testid={containerTestId}
				id={containerId}
				style={containerStyle}
			>
				{React.cloneElement(item, { id: radioId })}
				<RadioGroupLabel htmlFor={radioId} aria-disabled={isDisabled}>
					{children}
				</RadioGroupLabel>
			</div>
		);
	},
);
RadioGroupItem.displayName = 'RadioGroupItem';

export { RadioGroup, RadioGroupItem, RadioGroupLabel };
