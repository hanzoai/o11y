import './index.css';
import * as React from 'react';
import { cn } from '@hanzo/ui/core';
import { InputPassword } from './input-password';

export type InputTheme = 'light' | 'dark';

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
	theme?: InputTheme;
}

const InputComponent = React.forwardRef<HTMLInputElement, InputProps>(
	({ className, type, theme = 'light', ...props }, ref) => {
		return (
			<input
				type={type}
				data-slot="input"
				data-input-theme={theme}
				className={cn('hz-input', className)}
				ref={ref}
				{...props}
			/>
		);
	},
);
InputComponent.displayName = 'Input';

// Create compound component with proper typing
const Input = Object.assign(InputComponent, {
	Password: InputPassword,
}) as typeof InputComponent & {
	Password: typeof InputPassword;
};

export { Input, InputComponent, InputPassword };
