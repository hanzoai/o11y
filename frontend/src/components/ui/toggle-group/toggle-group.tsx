import './index.css';
import * as ToggleGroupPrimitive from '@radix-ui/react-toggle-group';
import React from 'react';
import { cn } from '../lib/utils';

export const ToggleColorValue = {
	Primary: 'primary',
	Destructive: 'destructive',
	Warning: 'warning',
	Secondary: 'secondary',
	None: 'none',
} as const;

export type ToggleColor =
	(typeof ToggleColorValue)[keyof typeof ToggleColorValue];

export type ToggleGroupProps = (
	| {
			type: 'single';
			/**
			 * The controlled stateful value of the item that is pressed.
			 */
			value?: string;
			/**
			 * The value of the item that is pressed when initially rendered. Use
			 * `defaultValue` if you do not need to control the state of a toggle group.
			 */
			defaultValue?: string;
			/**
			 * The callback that fires when the value of the toggle group changes.
			 */
			onChange?(value: string): void;
	  }
	| {
			type: 'multiple';
			/**
			 * The controlled stateful value of the items that are pressed.
			 */
			value?: string[];
			/**
			 * The value of the items that are pressed when initially rendered. Use
			 * `defaultValue` if you do not need to control the state of a toggle group.
			 */
			defaultValue?: string[];
			/**
			 * The callback that fires when the state of the toggle group changes.
			 */
			onChange?(value: string[]): void;
	  }
) & {
	/**
	 * Whether the group is disabled from user interaction.
	 * @defaultValue false
	 */
	disabled?: boolean;
	/**
	 * Whether the group should maintain roving focus of its buttons.
	 * @defaultValue true
	 */
	rovingFocus?: boolean;
	/**
	 * The loop of the toggle group.
	 */
	loop?: boolean;
	/**
	 * The orientation of the toggle group.
	 */
	orientation?: 'horizontal' | 'vertical';
	/**
	 * The direction of the toggle group.
	 */
	dir?: 'ltr' | 'rtl';
	/**
	 * The testId associated with the toggle group.
	 */
	testId?: string;
	/**
	 * The size of the toggle group.
	 * @default 'default'
	 */
	size?: 'default' | 'sm' | 'lg';
	/**
	 * The color of the toggle group.
	 * @default 'secondary'
	 */
	color?: ToggleColor;
} & Pick<
		React.ComponentPropsWithoutRef<'div'>,
		'id' | 'className' | 'style' | 'children'
	>;

/**
 * A set of two-state buttons that can be toggled on or off, in single or multiple selection mode.
 * Use ToggleGroupItem as children for full control over each option.
 */
export const ToggleGroup = React.forwardRef<HTMLDivElement, ToggleGroupProps>(
	(
		{
			className,
			children,
			size = 'default',
			color = 'secondary',
			// onChange is a React callback prop (mapped to Radix `onValueChange`), not a
			// class method — referencing it unbound is safe.
			// eslint-disable-next-line typescript-eslint/unbound-method
			onChange,
			testId,
			...props
		},
		ref,
	) => {
		const rootProps = {
			'data-slot': 'toggle-group',
			'data-size': size,
			'data-color': color,
			'data-testid': testId,
			className: cn(className),
			onValueChange: onChange,
			...props,
		} as unknown as React.ComponentPropsWithoutRef<
			typeof ToggleGroupPrimitive.Root
		>;
		return (
			<ToggleGroupPrimitive.Root ref={ref} {...rootProps}>
				{children}
			</ToggleGroupPrimitive.Root>
		);
	},
);
ToggleGroup.displayName = 'ToggleGroup';

export type ToggleGroupItemProps = {
	/**
	 * The value of the toggle group item.
	 */
	value: string;
	/**
	 * The testId associated with the toggle group item.
	 */
	testId?: string;
} & Pick<
	React.ComponentPropsWithoutRef<'button'>,
	| 'className'
	| 'style'
	| 'id'
	| 'disabled'
	| 'aria-disabled'
	| 'onClick'
	| 'children'
>;

/**
 * A single toggle option within ToggleGroup. Use as child of ToggleGroup.
 */
export const ToggleGroupItem = React.forwardRef<
	HTMLButtonElement,
	ToggleGroupItemProps
>(({ className, value, testId, ...props }, ref) => (
	<ToggleGroupPrimitive.Item
		ref={ref}
		data-slot="toggle-group-item"
		data-testid={testId}
		value={value}
		className={cn(className)}
		{...props}
	/>
));
ToggleGroupItem.displayName = 'ToggleGroupItem';
