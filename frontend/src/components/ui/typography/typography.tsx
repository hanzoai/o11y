import type React from 'react';
import { forwardRef, useCallback, useState } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { Check, Copy } from 'lucide-react';

import { cn } from '../lib/utils';

export type TypographySize =
	| 'small'
	| 'base'
	| 'medium'
	| 'large'
	| 'xs'
	| 'sm'
	| 'lg'
	| 'xl'
	| '2xl'
	| '3xl'
	| '4xl'
	| '5xl'
	| '6xl'
	| '7xl'
	| '8xl'
	| '9xl';
export type TypographyWeight =
	| 'thin'
	| 'extralight'
	| 'light'
	| 'normal'
	| 'medium'
	| 'semibold'
	| 'bold'
	| 'extrabold'
	| 'black';
export type TypographyElement =
	| 'h1'
	| 'h2'
	| 'h3'
	| 'h4'
	| 'h5'
	| 'h6'
	| 'p'
	| 'span'
	| 'div'
	| 'label'
	| 'a';
export type TypographyAlign = 'left' | 'center' | 'right';
export type TypographyVariant = 'title' | 'text';
export type TypographyColor = 'muted' | 'danger' | 'warning' | 'success';
export type TypographyLevel = 1 | 2 | 3 | 4 | 5;

export interface TypographyProps
	extends Pick<
		React.ComponentProps<'div'>,
		'children' | 'className' | 'id' | 'style' | 'title' | 'role' | 'tabIndex'
	> {
	onClick?: React.MouseEventHandler<unknown>;
	onMouseEnter?: React.MouseEventHandler<unknown>;
	onMouseLeave?: React.MouseEventHandler<unknown>;
	variant?: TypographyVariant;
	as?: TypographyElement;
	asChild?: boolean;
	size?: TypographySize;
	weight?: TypographyWeight;
	align?: TypographyAlign;
	truncate?: number;
	/** @deprecated use color="muted" */
	muted?: boolean;
	color?: TypographyColor;
	strong?: boolean;
	italic?: boolean;
	code?: boolean;
	disabled?: boolean;
	copyable?: boolean;
	level?: TypographyLevel;
	href?: string;
	target?: '_blank' | '_self' | '_parent' | '_top';
	rel?: string;
	testId?: string;
	interactive?: boolean;
}

const FONT_SIZE: Record<TypographySize, string> = {
	xs: '0.75rem',
	small: '0.75rem',
	sm: '0.875rem',
	base: '0.875rem',
	medium: '1rem',
	lg: '1.125rem',
	large: '1.125rem',
	xl: '1.25rem',
	'2xl': '1.5rem',
	'3xl': '1.875rem',
	'4xl': '2.25rem',
	'5xl': '3rem',
	'6xl': '3.75rem',
	'7xl': '4.5rem',
	'8xl': '6rem',
	'9xl': '8rem',
};

const FONT_WEIGHT: Record<TypographyWeight, number> = {
	thin: 100,
	extralight: 200,
	light: 300,
	normal: 400,
	medium: 500,
	semibold: 600,
	bold: 700,
	extrabold: 800,
	black: 900,
};

const LEVEL_SIZE: Record<TypographyLevel, TypographySize> = {
	1: '4xl',
	2: '3xl',
	3: '2xl',
	4: 'xl',
	5: 'lg',
};

const COLOR_VAR: Record<TypographyColor, string> = {
	muted: 'var(--text-muted-foreground, #6b7280)',
	danger: 'var(--bg-cherry-500, #e5484d)',
	warning: 'var(--bg-amber-500, #f5a623)',
	success: 'var(--bg-forest-500, #2da44e)',
};

function defaultElement(
	variant: TypographyVariant,
	level: TypographyLevel | undefined,
	href: string | undefined,
): TypographyElement {
	if (href) return 'a';
	if (variant === 'title') return (`h${level ?? 1}` as TypographyElement);
	return 'p';
}

const TypographyBase = forwardRef<HTMLElement, TypographyProps>((props, ref) => {
	const {
		children,
		className,
		style,
		variant = 'text',
		as,
		asChild = false,
		size,
		weight,
		align,
		truncate,
		muted = false,
		color,
		strong = false,
		italic = false,
		code = false,
		disabled = false,
		copyable = false,
		level,
		href,
		target,
		rel,
		testId,
		interactive = false,
		onClick,
		...rest
	} = props;

	const [copied, setCopied] = useState(false);
	const handleCopy = useCallback(() => {
		const text = typeof children === 'string' ? children : String(children ?? '');
		navigator.clipboard?.writeText(text).then(() => {
			setCopied(true);
			setTimeout(() => setCopied(false), 1500);
		});
	}, [children]);

	const resolvedSize = size ?? (variant === 'title' && level ? LEVEL_SIZE[level] : 'base');
	const resolvedColor = color ?? (muted ? 'muted' : undefined);
	const isInteractive = interactive || Boolean(onClick);

	const computedStyle: React.CSSProperties = {
		fontSize: FONT_SIZE[resolvedSize],
		...(weight ? { fontWeight: FONT_WEIGHT[weight] } : null),
		...(strong ? { fontWeight: FONT_WEIGHT.semibold } : null),
		...(variant === 'title' && !weight && !strong ? { fontWeight: FONT_WEIGHT.semibold } : null),
		...(align ? { textAlign: align } : null),
		...(italic ? { fontStyle: 'italic' } : null),
		...(resolvedColor ? { color: COLOR_VAR[resolvedColor] } : null),
		...(disabled ? { opacity: 0.5, pointerEvents: 'none' } : null),
		...(isInteractive ? { cursor: 'pointer' } : null),
		...(truncate
			? truncate === 1
				? { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }
				: {
						display: '-webkit-box',
						WebkitLineClamp: truncate,
						WebkitBoxOrient: 'vertical',
						overflow: 'hidden',
				  }
			: null),
		...style,
	};

	const Element: React.ElementType = asChild
		? Slot
		: as ?? defaultElement(variant, level, href);

	const anchorProps = href ? { href, target, rel } : {};

	const content = (
		<Element
			ref={ref as never}
			className={cn('hz-typography', code && 'hz-typography--code', className)}
			style={computedStyle}
			data-slot="typography"
			data-variant={variant}
			data-testid={testId}
			onClick={onClick}
			{...anchorProps}
			{...rest}
		>
			{children}
		</Element>
	);

	if (!copyable) return content;
	return (
		<span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
			{content}
			<button
				type="button"
				aria-label="Copy"
				onClick={handleCopy}
				style={{
					background: 'none',
					border: 'none',
					cursor: 'pointer',
					padding: 0,
					display: 'inline-flex',
					color: 'inherit',
				}}
			>
				{copied ? <Check size={14} /> : <Copy size={14} />}
			</button>
		</span>
	);
});
TypographyBase.displayName = 'Typography';

export interface TypographyTextProps extends Omit<TypographyProps, 'variant' | 'level'> {}
const TypographyText = forwardRef<HTMLElement, TypographyTextProps>((props, ref) => (
	<TypographyBase ref={ref} variant="text" {...props} />
));
TypographyText.displayName = 'Typography.Text';

export interface TypographyTitleProps extends Omit<TypographyProps, 'variant'> {}
const TypographyTitle = forwardRef<HTMLElement, TypographyTitleProps>((props, ref) => (
	<TypographyBase ref={ref} variant="title" {...props} />
));
TypographyTitle.displayName = 'Typography.Title';

export interface TypographyLinkProps extends Omit<TypographyProps, 'variant' | 'level'> {
	href?: string;
}
const TypographyLink = forwardRef<HTMLElement, TypographyLinkProps>((props, ref) => (
	<TypographyBase ref={ref} variant="text" {...props} />
));
TypographyLink.displayName = 'Typography.Link';

type TypographyComponent = typeof TypographyBase & {
	Text: typeof TypographyText;
	Title: typeof TypographyTitle;
	Link: typeof TypographyLink;
};

export const Typography = Object.assign(TypographyBase, {
	Text: TypographyText,
	Title: TypographyTitle,
	Link: TypographyLink,
}) as TypographyComponent;
