import './index.css';
import * as React from 'react';
import { cn } from '../lib/utils';

export interface DividerProps
	extends Pick<React.ComponentProps<'div'>, 'className' | 'children' | 'id' | 'style'> {
	type?: 'horizontal' | 'vertical';
	dashed?: boolean;
	plain?: boolean;
	testId?: string;
}

export const Divider = React.forwardRef<HTMLDivElement, DividerProps>(
	({ className, type = 'horizontal', dashed = false, plain = false, testId, children, ...props }, ref) => {
		const hasChildren = children != null;
		return (
			<div
				ref={ref}
				{...(hasChildren ? {} : { role: 'separator', 'aria-orientation': type })}
				data-slot="divider"
				data-type={type}
				data-dashed={dashed || undefined}
				data-plain={plain || undefined}
				data-with-text={hasChildren || undefined}
				data-testid={testId}
				className={cn('divider', className)}
				{...props}
			>
				{hasChildren && (
					<span data-slot="divider-text" className="divider-text">
						{children}
					</span>
				)}
			</div>
		);
	}
);
Divider.displayName = 'Divider';
