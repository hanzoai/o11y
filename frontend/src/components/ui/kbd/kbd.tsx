import './index.css';
import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cn } from '../lib/utils';

export type KbdSize = 'sm' | 'default' | 'lg';

export interface KbdProps extends Pick<
	React.ComponentProps<'kbd'>,
	'className' | 'children' | 'id' | 'style'
> {
	/**
	 * The testId associated with the kbd element.
	 */
	testId?: string;
	/**
	 * @default false
	 */
	asChild?: boolean;
	/**
	 * @default default
	 */
	size?: KbdSize;
	/**
	 * Highlights the key with a subtle primary color tint.
	 * @default false
	 */
	active?: boolean;
}

export const Kbd = React.forwardRef<HTMLElement, KbdProps>(
	(
		{
			className,
			size = 'default',
			asChild = false,
			active = false,
			testId,
			children,
			...props
		},
		ref,
	) => {
		const Comp = asChild ? Slot : 'kbd';
		return (
			<Comp
				ref={ref}
				data-slot="kbd"
				data-size={size}
				data-active={active || undefined}
				data-testid={testId}
				className={cn('kbd', className)}
				{...props}
			>
				{children}
			</Comp>
		);
	},
);
Kbd.displayName = 'Kbd';
