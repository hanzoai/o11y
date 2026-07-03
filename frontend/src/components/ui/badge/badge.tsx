import './index.css';
import type { MouseEvent, ReactNode } from 'react';
import { forwardRef, useState } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { X } from 'lucide-react';

import { cn } from '../lib/utils';

export type BadgeVariant = 'default' | 'outline';
export type BadgeColor =
	| 'primary'
	| 'secondary'
	| 'success'
	| 'error'
	| 'warning'
	| 'robin'
	| 'forest'
	| 'amber'
	| 'sienna'
	| 'cherry'
	| 'sakura'
	| 'aqua'
	| 'vanilla';
export type TextEllipsisPosition = 'start' | 'center' | 'end';

export interface BadgeProps
	extends Pick<React.ComponentProps<'span'>, 'className' | 'children' | 'id' | 'style'> {
	testId?: string;
	asChild?: boolean;
	variant?: BadgeVariant;
	color?: BadgeColor;
	capitalize?: boolean;
	textEllipsis?: boolean | TextEllipsisPosition;
	closable?: boolean;
	onClose?: (event: MouseEvent<HTMLButtonElement>) => void;
	closeIcon?: ReactNode;
	closeAriaLabel?: string;
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>((props, ref) => {
	const {
		className,
		children,
		style,
		testId,
		asChild = false,
		variant = 'default',
		color = 'primary',
		capitalize = false,
		textEllipsis = false,
		closable = false,
		onClose,
		closeIcon,
		closeAriaLabel = 'Close badge',
		...rest
	} = props;

	const [hidden, setHidden] = useState(false);
	if (hidden) return null;

	const Comp: React.ElementType = asChild ? Slot : 'span';
	const truncate = textEllipsis !== false;

	const handleClose = (event: MouseEvent<HTMLButtonElement>): void => {
		onClose?.(event);
		if (!event.defaultPrevented) setHidden(true);
	};

	return (
		<Comp
			ref={ref as never}
			className={cn('hz-badge', capitalize && 'hz-badge--capitalize', className)}
			data-slot="badge"
			data-variant={variant}
			data-color={color}
			data-testid={testId}
			style={style}
			{...rest}
		>
			{truncate && typeof children === 'string' ? (
				<span className="hz-badge__ellipsis">{children}</span>
			) : (
				children
			)}
			{closable && !asChild && (
				<button
					type="button"
					className="hz-badge__close"
					aria-label={closeAriaLabel}
					onClick={handleClose}
				>
					{closeIcon ?? <X size={12} />}
				</button>
			)}
		</Comp>
	);
});
Badge.displayName = 'Badge';
