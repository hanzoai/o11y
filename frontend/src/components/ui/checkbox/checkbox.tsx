import './index.css';
import * as CheckboxPrimitive from '@radix-ui/react-checkbox';
import type { CheckedState } from '@radix-ui/react-checkbox';
import { Check, Slash } from 'lucide-react';
import React from 'react';
import { cn } from '../lib/utils';

type CheckboxColor =
	| 'primary'
	| 'success'
	| 'warning'
	| 'error'
	| 'robin'
	| 'forest'
	| 'amber'
	| 'sienna'
	| 'cherry'
	| 'sakura'
	| 'aqua';

const colorMap: Record<string, CheckboxColor> = {
	success: 'forest',
	warning: 'amber',
	error: 'cherry',
	primary: 'robin',
};

export const CheckboxColors: Record<
	Capitalize<CheckboxColor>,
	CheckboxColor
> = {
	Primary: 'primary',
	Success: 'success',
	Warning: 'warning',
	Error: 'error',
	Robin: 'robin',
	Forest: 'forest',
	Amber: 'amber',
	Sienna: 'sienna',
	Cherry: 'cherry',
	Sakura: 'sakura',
	Aqua: 'aqua',
};

export interface CheckboxProps extends Pick<
	React.ComponentPropsWithoutRef<'button'>,
	'id' | 'disabled' | 'className' | 'children' | 'onClick'
> {
	/**
	 * The name of the checkbox. Submitted with its owning form as part of a name/value pair.
	 */
	name?: string;
	/**
	 * The color of the checkbox.
	 * @default primary
	 */
	color?: CheckboxColor;
	/**
	 * The value given as data when submitted with a name.
	 */
	value?: CheckedState;
	/**
	 * The checked state of the checkbox when it is initially rendered. Use when you do not need to control its checked state.
	 * @default undefined
	 */
	defaultValue?: CheckedState;
	/**
	 * When true, indicates that the user must check the checkbox before the owning form can be submitted.
	 * @default false
	 */
	required?: boolean;
	/**
	 * The testId associated with the checkbox.
	 */
	testId?: string;
	/**
	 * The callback invoked when the value state of the checkbox changes.
	 * @param checked
	 */
	onChange?(checked: CheckedState): void;
}

const CheckboxBase = React.forwardRef<
	HTMLButtonElement,
	Omit<CheckboxProps, 'testId'>
>(
	(
		{ className, color = 'primary', onChange, value, defaultValue, ...props },
		ref,
	) => (
		<CheckboxPrimitive.Root
			ref={ref}
			data-slot="checkbox"
			data-color={colorMap[color] || color}
			className={cn(className)}
			checked={value}
			defaultChecked={defaultValue}
			onCheckedChange={onChange}
			{...props}
		>
			<CheckboxPrimitive.Indicator data-slot="checkbox-indicator">
				<Slash data-slot="checkbox-icon-slash" />
				<Check data-slot="checkbox-icon-check" />
			</CheckboxPrimitive.Indicator>
		</CheckboxPrimitive.Root>
	),
);
CheckboxBase.displayName = CheckboxPrimitive.Root.displayName;

const CheckboxWrapper = React.forwardRef<HTMLButtonElement, CheckboxProps>(
	({ id, children, testId, className, ...props }, ref) => {
		const fallbackId = React.useId();
		return (
			<div
				data-slot="checkbox-wrapper"
				className={cn(className)}
				data-testid={testId}
			>
				<CheckboxBase ref={ref} id={id || fallbackId} {...props} />
				{children && (
					<label htmlFor={id || fallbackId} data-slot="checkbox-label">
						{children}
					</label>
				)}
			</div>
		);
	},
);
CheckboxWrapper.displayName = 'Checkbox';

export { CheckboxWrapper as Checkbox };
