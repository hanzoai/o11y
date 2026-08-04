import './index.css';
import {
	TriangleAlert,
	CircleCheck,
	Info,
	CircleX,
	X,
	type LucideProps,
} from 'lucide-react';
import React from 'react';
import { cn } from '@hanzo/ui/core';

interface CalloutProps extends React.ComponentProps<'div'> {
	message?: React.ReactNode;
	description?: React.ReactNode;
	type?: 'info' | 'success' | 'warning' | 'error';
	showIcon?: boolean;
	icon?: React.ReactNode;
	color?: string;
	size?: 'small' | 'medium';
	dismissable?: boolean;
	onClose?: () => void;
}

const typeToColorMap = {
	info: 'robin',
	success: 'forest',
	warning: 'amber',
	error: 'cherry',
} as const;

const defaultIcons = {
	info: <Info />,
	success: <CircleCheck />,
	warning: <TriangleAlert />,
	error: <CircleX />,
};

function Callout({
	className,
	message,
	description,
	type = 'info',
	showIcon = false,
	icon,
	color,
	size = 'small',
	dismissable = false,
	onClose,
	...props
}: CalloutProps) {
	const IconComponent = icon || (showIcon && defaultIcons[type]);

	return (
		<div
			data-slot="callout"
			data-color={color ?? typeToColorMap[type]}
			data-size={size}
			role="alert"
			className={cn('hz-callout', className)}
			{...props}
		>
			{IconComponent ? (
				React.isValidElement<LucideProps>(IconComponent) ? (
					React.cloneElement(IconComponent, {
						'aria-hidden': true,
						className: cn('hz-callout__icon', IconComponent.props.className),
						color: 'var(--callout-icon-color)',
						size: size === 'medium' ? 16 : 12,
					})
				) : (
					<span className="hz-callout__icon">{IconComponent}</span>
				)
			) : (
				<div className="hz-callout__icon-placeholder" />
			)}
			<div className="hz-callout__body">
				{message && (
					<div data-slot="callout-title" className="hz-callout__title">
						{message}
					</div>
				)}
				{description && (
					<div
						data-slot="callout-description"
						className="hz-callout__description"
					>
						{description}
					</div>
				)}
			</div>
			{dismissable && (
				<button
					type="button"
					aria-label="Close"
					onClick={onClose}
					className="hz-callout__dismiss"
				>
					<X size={size === 'medium' ? 16 : 14} />
				</button>
			)}
		</div>
	);
}

export { Callout, type CalloutProps };
