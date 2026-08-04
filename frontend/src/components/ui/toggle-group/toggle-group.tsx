import './index.css';
import React from 'react';
import { cn } from '@hanzo/ui/core';

import { focusFirstItem, rovingKeyDown } from '../lib/roving-focus';
import { useControllable } from '../lib/use-controllable';

interface ToggleGroupContextValue {
	pressed: (value: string) => boolean;
	toggle: (value: string) => void;
	disabled?: boolean;
	roving: boolean;
	anyPressed: boolean;
	dir: 'ltr' | 'rtl';
}

const ToggleGroupContext = React.createContext<ToggleGroupContextValue | null>(
	null,
);

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
		const {
			type,
			value,
			defaultValue,
			disabled,
			rovingFocus = true,
			orientation,
			dir = 'ltr',
			// `loop` is radix vocabulary no call site sets: navigation always wraps.
			loop: _loop,
			...rest
		} = props as Omit<ToggleGroupProps, 'onChange' | 'children'> & {
			type: 'single' | 'multiple';
			value?: string | string[];
			defaultValue?: string | string[];
		};

		const multiple = type === 'multiple';
		const empty = multiple ? [] : null;
		const [selection, setSelection] = useControllable<string | string[] | null>(
			value ?? undefined,
			defaultValue ?? empty,
			onChange as ((next: string | string[] | null) => void) | undefined,
		);

		const context = React.useMemo<ToggleGroupContextValue>(() => {
			const list = multiple ? (selection as string[]) : [];
			return {
				pressed: (item): boolean =>
					multiple ? list.includes(item) : selection === item,
				toggle: (item): void => {
					if (multiple) {
						setSelection(
							list.includes(item) ? list.filter((v) => v !== item) : [...list, item],
						);
					} else {
						// Radix's single-mode rule: pressing the pressed item clears it.
						setSelection(selection === item ? '' : item);
					}
				},
				disabled,
				roving: rovingFocus,
				anyPressed: multiple ? list.length > 0 : Boolean(selection),
				dir,
			};
		}, [multiple, selection, setSelection, disabled, rovingFocus, dir]);

		return (
			<ToggleGroupContext.Provider value={context}>
				<div
					ref={ref}
					role="group"
					dir={dir}
					// role="group" takes no aria-orientation — only a toolbar, listbox or
					// radiogroup does. The orientation is ours to style on, not to announce.
					data-orientation={orientation}
					data-slot="toggle-group"
					data-size={size}
					data-color={color}
					data-testid={testId}
					data-roving-group={rovingFocus ? '' : undefined}
					className={cn(className)}
					tabIndex={rovingFocus && !context.anyPressed ? 0 : -1}
					onFocus={focusFirstItem}
					{...rest}
				>
					{children}
				</div>
			</ToggleGroupContext.Provider>
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
>(({ className, value, testId, onClick, disabled, ...props }, ref) => {
	const group = React.useContext(ToggleGroupContext);
	const pressed = group?.pressed(value) ?? false;
	const isDisabled = disabled || group?.disabled;

	return (
		<button
			ref={ref}
			type="button"
			aria-pressed={pressed}
			data-slot="toggle-group-item"
			data-state={pressed ? 'on' : 'off'}
			data-roving-item=""
			data-testid={testId}
			value={value}
			disabled={isDisabled}
			className={cn(className)}
			tabIndex={!group?.roving || pressed ? 0 : -1}
			onClick={(event): void => {
				onClick?.(event);
				group?.toggle(value);
			}}
			onKeyDown={(event): void => {
				if (group?.roving) {
					rovingKeyDown(event, undefined, group.dir);
				}
			}}
			{...props}
		/>
	);
});
ToggleGroupItem.displayName = 'ToggleGroupItem';
