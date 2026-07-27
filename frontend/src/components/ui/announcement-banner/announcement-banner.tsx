import './index.css';
import { CircleAlert, CircleCheckBig, Info, TriangleAlert, X } from 'lucide-react';
import { type ComponentPropsWithoutRef, forwardRef, type ReactNode } from 'react';
import { cn } from '../lib/utils';

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
} & Pick<ComponentPropsWithoutRef<'div'>, 'id' | 'className' | 'style' | 'children'>;

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
	({ children, type = 'warning', icon, action, onClose, className, style, testId, id }, ref) => {
		const resolvedIcon = icon === null ? null : (icon ?? DEFAULT_ICONS[type]);

		return (
			<div
				id={id}
				role="alert"
				ref={ref}
				data-testid={testId}
				data-type={type}
				data-slot="announcement-banner"
				className={cn(
					'flex items-center justify-between gap-2 px-4 py-2 text-[13px] font-medium leading-5 tracking-[-0.065px]',
					className
				)}
				style={style}
			>
				<div
					data-slot="announcement-banner-body"
					className="flex min-w-0 flex-1 items-center gap-2"
				>
					{resolvedIcon && (
						<span
							data-slot="announcement-banner-icon"
							data-testid="banner-icon"
							className="flex shrink-0 items-center"
						>
							{resolvedIcon}
						</span>
					)}
					<div
						data-slot="announcement-banner-message"
						className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
					>
						{children}
					</div>
					{action && (
						<button
							type="button"
							data-slot="announcement-banner-action"
							className="inline-flex h-6 shrink-0 cursor-pointer items-center rounded-[2px] bg-[var(--banner-accent)] px-2 text-xs font-medium text-[var(--banner-accent-fg)] transition-opacity hover:opacity-90"
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
						className="inline-flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded-[2px] bg-[var(--banner-accent)] text-[var(--banner-accent-fg)] transition-opacity hover:opacity-90"
						onClick={onClose}
					>
						<X size={14} />
					</button>
				)}
			</div>
		);
	}
);
AnnouncementBanner.displayName = 'AnnouncementBanner';

export { AnnouncementBanner };
