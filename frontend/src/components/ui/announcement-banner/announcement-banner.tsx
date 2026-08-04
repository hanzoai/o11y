import './index.css';
import {
	CircleAlert,
	CircleCheckBig,
	Info,
	TriangleAlert,
	X,
} from 'lucide-react';
import {
	type ComponentPropsWithoutRef,
	forwardRef,
	type ReactNode,
} from 'react';
import { cn } from '@hanzo/ui/core';

export type AnnouncementBannerType = 'warning' | 'info' | 'error' | 'success';

export type AnnouncementBannerAction = {
	/**
	 * The label of the action.
	 */
	label: ReactNode;
	/**
	 * The callback to call when the action is clicked.
	 */
	onClick: () => void;
};

export type AnnouncementBannerProps = {
	/**
	 * The type of banner to display.
	 */
	type?: AnnouncementBannerType;
	/**
	 * The icon to display in the banner.
	 */
	icon?: ReactNode | null;
	/**
	 * The action to display in the banner.
	 */
	action?: AnnouncementBannerAction;
	/**
	 * The callback to call when the banner is closed.
	 */
	onClose?: () => void;
	/**
	 * The test id to apply to the banner.
	 */
	testId?: string;
} & Pick<
	ComponentPropsWithoutRef<'div'>,
	'id' | 'className' | 'style' | 'children'
>;

const DEFAULT_ICONS: Record<AnnouncementBannerType, ReactNode> = {
	warning: <TriangleAlert size={14} />,
	info: <Info size={14} />,
	error: <CircleAlert size={14} />,
	success: <CircleCheckBig size={14} />,
};

/**
 * A banner component for displaying announcements, alerts, or notices.
 */
const AnnouncementBanner = forwardRef<HTMLDivElement, AnnouncementBannerProps>(
	(
		{
			children,
			type = 'warning',
			icon,
			action,
			onClose,
			className,
			style,
			testId,
			id,
		},
		ref,
	) => {
		const resolvedIcon = icon === null ? null : (icon ?? DEFAULT_ICONS[type]);

		return (
			<div
				id={id}
				role="alert"
				ref={ref}
				data-testid={testId}
				data-type={type}
				data-slot="announcement-banner"
				className={cn(className)}
				style={style}
			>
				<div
					data-slot="announcement-banner-body"
				>
					{resolvedIcon && (
						<span
							data-slot="announcement-banner-icon"
							data-testid="banner-icon"
						>
							{resolvedIcon}
						</span>
					)}
					<div
						data-slot="announcement-banner-message"
					>
						{children}
					</div>
					{action && (
						<button
							type="button"
							data-slot="announcement-banner-action"
							onClick={action.onClick}
						>
							{action.label}
						</button>
					)}
				</div>
				{onClose && (
					<button
						type="button"
						data-slot="announcement-banner-dismiss"
						aria-label="Dismiss"
						onClick={onClose}
					>
						<X size={14} />
					</button>
				)}
			</div>
		);
	},
);
AnnouncementBanner.displayName = 'AnnouncementBanner';

export { AnnouncementBanner };
