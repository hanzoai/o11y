import './index.css';
import * as SwitchPrimitive from '@radix-ui/react-switch';
import React from 'react';
import { cn } from '../lib/utils';

type SwitchColor =
	| 'robin'
	| 'forest'
	| 'amber'
	| 'sienna'
	| 'cherry'
	| 'sakura'
	| 'aqua';

export type SwitchProps = Pick<
	React.ComponentPropsWithoutRef<'button'>,
	'id' | 'className' | 'style' | 'children' | 'name'
> & {
	/**
	 * The testId associated with the switch.
	 */
	testId?: string;
	/**
	 * Additional CSS classes to apply to the switch wrapper.
	 */
	containerClassName?: string;
	/**
	 * Inline styles to apply to the switch wrapper.
	 */
	containerStyle?: React.CSSProperties;
	/**
	 * The id of the switch wrapper.
	 */
	containerId?: string;
	/**
	 * The testId associated with the switch wrapper.
	 */
	containerTestId?: string;
	/**
	 * The controlled checked state of the switch. Must be used in conjunction with onChange.
	 */
	value?: boolean;
	/**
	 * The initial checked state of the switch. Use when you do not need to control its state.
	 */
	defaultValue?: boolean;
	/**
	 * Whether the switch is disabled. When true, the user cannot interact with it.
	 */
	disabled?: boolean;
	/**
	 * When true, indicates that the user must toggle the switch before the owning form can be submitted.
	 */
	required?: boolean;
	/**
	 * Event handler called when the checked state of the switch changes.
	 */
	onChange?(checked: boolean): void;
	/**
	 * The color variant of the switch.
	 *
	 * @default 'robin'
	 */
	color?: SwitchColor;
};

type SwitchBaseProps = Omit<
	SwitchProps,
	'containerClassName' | 'containerStyle' | 'containerId' | 'containerTestId'
>;

const SwitchBase = React.forwardRef<HTMLButtonElement, SwitchBaseProps>(
	(
		{
			className,
			style,
			testId,
			value,
			onChange,
			defaultValue,
			color = 'robin',
			...props
		},
		ref,
	) => (
		<SwitchPrimitive.Root
			ref={ref}
			data-slot="switch"
			data-color={color}
			className={cn(className)}
			data-testid={testId}
			style={style}
			checked={value}
			onCheckedChange={onChange}
			defaultChecked={defaultValue}
			{...props}
		>
			<SwitchPrimitive.Thumb data-slot="switch-thumb" />
		</SwitchPrimitive.Root>
	),
);
SwitchBase.displayName = SwitchPrimitive.Root.displayName;

/**
 * A toggle switch component for binary on/off or true/false selections.
 */
const SwitchWrapper = React.forwardRef<HTMLButtonElement, SwitchProps>(
	(
		{
			children,
			id,
			testId,
			className,
			style,
			containerClassName,
			containerStyle,
			containerId,
			containerTestId,
			...props
		},
		ref,
	) => {
		const fallbackId = React.useId();
		const switchId = id || fallbackId;
		return (
			<div
				data-slot="switch-wrapper"
				className={cn(containerClassName)}
				data-testid={containerTestId}
				id={containerId}
				style={containerStyle}
			>
				<SwitchBase
					ref={ref}
					id={switchId}
					testId={testId}
					className={className}
					style={style}
					{...props}
				/>
				{children && (
					<label htmlFor={switchId} data-slot="switch-label">
						{children}
					</label>
				)}
			</div>
		);
	},
);
SwitchWrapper.displayName = 'Switch';

export { SwitchWrapper as Switch };
