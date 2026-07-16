import './index.css';
import * as React from 'react';
import { cn } from '../lib/utils';

export type AvatarSize = 'sm' | 'md' | 'lg' | 'xl';

export type AvatarColor =
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

export interface AvatarProps extends Pick<
	React.ComponentProps<'span'>,
	'className' | 'children' | 'id' | 'style'
> {
	size?: AvatarSize;
	src?: string;
	alt?: string;
	shape?: 'circle' | 'square';
	color?: AvatarColor;
	loading?: boolean;
	testId?: string;
}

const colorMap: Partial<Record<AvatarColor, AvatarColor>> = {
	success: 'forest',
	warning: 'amber',
	error: 'cherry',
	primary: 'robin',
	secondary: 'vanilla',
};

export const Avatar = React.forwardRef<HTMLSpanElement, AvatarProps>(
	(
		{
			className,
			size = 'md',
			src,
			alt,
			shape = 'circle',
			color,
			loading = false,
			testId,
			children,
			...props
		},
		ref,
	) => {
		const [imgError, setImgError] = React.useState(false);
		const resolvedColor = color ? (colorMap[color] ?? color) : undefined;
		return (
			<span
				ref={ref}
				data-slot="avatar"
				data-size={size}
				data-shape={shape}
				data-color={resolvedColor}
				data-loading={loading || undefined}
				data-testid={testId}
				className={cn('avatar', className)}
				{...props}
			>
				{loading ? (
					<span data-slot="avatar-skeleton" className="avatar-skeleton" />
				) : src && !imgError ? (
					<img
						data-slot="avatar-image"
						className="avatar-image"
						src={src}
						alt={alt}
						onError={(): void => setImgError(true)}
					/>
				) : (
					<span data-slot="avatar-fallback" className="avatar-fallback">
						{children}
					</span>
				)}
			</span>
		);
	},
);
Avatar.displayName = 'Avatar';
