import { EyeIcon, EyeOffIcon } from 'lucide-react';
import * as React from 'react';
import { Button } from '@hanzo/ui';
import { cn } from '@hanzo/ui/core';
import { InputComponent, type InputTheme } from './input';

export interface InputPasswordProps
	extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type'> {
	theme?: InputTheme;
}

const InputPassword = React.forwardRef<HTMLInputElement, InputPasswordProps>(
	({ className, theme, ...props }, ref) => {
		const [showPassword, setShowPassword] = React.useState(false);

		const togglePasswordVisibility = () => {
			setShowPassword((prev) => !prev);
		};

		return (
			<div className="hz-input-password">
				<InputComponent
					type={showPassword ? 'text' : 'password'}
					className={cn(className)}
					theme={theme}
					ref={ref}
					{...props}
				/>
				<Button
					type="button"
					variant="ghost"
					size="icon"
					onClick={togglePasswordVisibility}
					className="hz-input-password__toggle"
					aria-label={showPassword ? 'Hide password' : 'Show password'}
					tabIndex={-1}
					disabled={props.disabled}
				>
					{showPassword ? (
						<EyeOffIcon size={16} aria-hidden="true" strokeWidth={2} />
					) : (
						<EyeIcon size={16} aria-hidden="true" strokeWidth={2} />
					)}
				</Button>
			</div>
		);
	},
);
InputPassword.displayName = 'InputPassword';

export { InputPassword };
