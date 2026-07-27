import './index.css';
import type React from 'react';
import { createContext, forwardRef, useContext } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { LoaderCircle } from 'lucide-react';

import { cn } from '../lib/utils';

export const ButtonVariant = {
	Solid: 'solid',
	Outlined: 'outlined',
	Dashed: 'dashed',
	Ghost: 'ghost',
	Link: 'link',
	Action: 'action',
} as const;
export const ButtonSize = { SM: 'sm', MD: 'md', Icon: 'icon' } as const;
export const ButtonBackground = {
	Ink500: 'ink-500',
	Ink400: 'ink-400',
	Vanilla100: 'vanilla-100',
	Vanilla200: 'vanilla-200',
} as const;
export const ButtonColor = {
	Primary: 'primary',
	Destructive: 'destructive',
	Warning: 'warning',
	Secondary: 'secondary',
	None: 'none',
} as const;

export type ButtonVariantValue = (typeof ButtonVariant)[keyof typeof ButtonVariant];
export type ButtonSizeValue = (typeof ButtonSize)[keyof typeof ButtonSize];
export type ButtonBackgroundValue = (typeof ButtonBackground)[keyof typeof ButtonBackground];
export type ButtonColorValue = (typeof ButtonColor)[keyof typeof ButtonColor] | (string & {});

export interface ButtonGroupContextValue {
	size?: ButtonSizeValue;
	variant?: ButtonVariantValue;
	color?: ButtonColorValue;
	inGroup: boolean;
}
export const ButtonGroupContext = createContext<ButtonGroupContextValue | null>(null);

export function buttonVariants({
	variant = 'solid',
	size = 'md',
	className,
}: {
	variant?: ButtonVariantValue;
	size?: ButtonSizeValue;
	className?: string;
} = {}): string {
	return cn('hz-btn', `hz-btn--${variant}`, `hz-btn--${size}`, className);
}

export type ButtonProps = {
	variant?: ButtonVariantValue;
	size?: ButtonSizeValue;
	asChild?: boolean;
	color?: ButtonColorValue;
	prefix?: React.ReactElement;
	suffix?: React.ReactElement;
	loading?: boolean;
	background?: ButtonBackgroundValue;
	testId?: string;
} & Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'prefix' | 'color'>;

export const Button = forwardRef<HTMLButtonElement, ButtonProps>((props, ref) => {
	const group = useContext(ButtonGroupContext);
	const {
		variant = group?.variant ?? 'solid',
		size = group?.size ?? 'md',
		asChild = false,
		color = group?.color ?? 'primary',
		prefix,
		suffix,
		loading = false,
		background,
		testId,
		className,
		children,
		disabled,
		type = 'button',
		...rest
	} = props;

	const classNames = cn(buttonVariants({ variant, size }), className);

	if (asChild) {
		return (
			<Slot
				ref={ref as never}
				className={classNames}
				data-slot="button"
				data-variant={variant}
				data-color={color}
				data-size={size}
				data-testid={testId}
				{...rest}
			>
				{children}
			</Slot>
		);
	}

	return (
		<button
			ref={ref}
			// eslint-disable-next-line react/button-has-type
			type={type}
			className={classNames}
			data-slot="button"
			data-variant={variant}
			data-color={color}
			data-size={size}
			data-background={variant === 'action' ? background : undefined}
			data-testid={testId}
			disabled={disabled || loading}
			{...rest}
		>
			{loading ? (
				<LoaderCircle className="hz-btn__spinner" size={14} />
			) : (
				prefix && <span className="hz-btn__affix">{prefix}</span>
			)}
			{children}
			{!loading && suffix && <span className="hz-btn__affix">{suffix}</span>}
		</button>
	);
});
Button.displayName = 'Button';
