import type React from 'react';
import { forwardRef } from 'react';

import { cn } from '../lib/utils';

export type DialogSubtitleProps = Pick<
	React.ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
> & {
	testId?: string;
};

export const DialogSubtitle = forwardRef<HTMLDivElement, DialogSubtitleProps>(
	({ className, testId, children, ...props }, ref) => (
		<div
			ref={ref}
			data-slot="dialog-subtitle"
			data-testid={testId}
			className={cn('px-4 text-sm text-l2-foreground', className)}
			{...props}
		>
			{children}
		</div>
	),
);
DialogSubtitle.displayName = 'DialogSubtitle';
