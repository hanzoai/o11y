import type React from 'react';
import { forwardRef } from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';

import { Button, type ButtonProps } from '../button';

export interface DialogCloseButtonProps extends Omit<ButtonProps, 'type' | 'aria-label'> {
	ariaLabel?: string;
	icon?: React.ReactNode;
}

export const DialogCloseButton = forwardRef<HTMLButtonElement, DialogCloseButtonProps>(
	({ ariaLabel = 'Close', icon, variant = 'ghost', size = 'icon', color = 'secondary', ...props }, ref) => (
		<DialogPrimitive.Close asChild>
			<Button
				ref={ref}
				variant={variant}
				size={size}
				color={color}
				aria-label={ariaLabel}
				{...props}
			>
				{icon ?? <X size={14} />}
			</Button>
		</DialogPrimitive.Close>
	),
);
DialogCloseButton.displayName = 'DialogCloseButton';
