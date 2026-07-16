import './index.css';
import * as RadioGroupPrimitive from '@radix-ui/react-radio-group';
import React from 'react';
import { cn } from '../lib/utils';

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
	({ className, onChange, color = 'robin', testId, ...props }, ref) => (
		<RadioGroupPrimitive.Root
			ref={ref}
			data-slot="radio-group"
			data-color={color}
			data-testid={testId}
			className={cn(className)}
			onValueChange={onChange}
			{...(props as React.ComponentPropsWithoutRef<
				typeof RadioGroupPrimitive.Root
			>)}
		/>
	),
);
RadioGroup.displayName = RadioGroupPrimitive.Root.displayName;

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
		const fallbackId = React.useId();
		const radioId = props.id || fallbackId;
		if (children) {
			return (
				<div
					data-slot="radio-group-item-wrapper"
					className={cn(containerClassName)}
					data-testid={containerTestId}
					id={containerId}
					style={containerStyle}
				>
					<RadioGroupPrimitive.Item
						ref={ref}
						data-slot="radio-group-item"
						className={cn(className)}
						id={radioId}
						data-testid={testId}
						style={style}
						{...props}
					>
						<RadioGroupPrimitive.Indicator data-slot="radio-group-indicator" />
					</RadioGroupPrimitive.Item>
					<RadioGroupLabel htmlFor={radioId} aria-disabled={props.disabled}>
						{children}
					</RadioGroupLabel>
				</div>
			);
		}
		return (
			<RadioGroupPrimitive.Item
				ref={ref}
				data-slot="radio-group-item"
				className={cn(className)}
				data-testid={testId}
				style={style}
				{...props}
			>
				<RadioGroupPrimitive.Indicator data-slot="radio-group-indicator" />
			</RadioGroupPrimitive.Item>
		);
	},
);
RadioGroupItem.displayName = RadioGroupPrimitive.Item.displayName;

export { RadioGroup, RadioGroupItem, RadioGroupLabel };
