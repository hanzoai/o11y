import getLocalStorage from 'api/browser/localstorage/get';
import setLocalStorage from 'api/browser/localstorage/set';
import { forwardRef, useCallback, useState } from 'react';
import { AnnouncementBanner, type AnnouncementBannerProps } from './announcement-banner';

export type PersistedAnnouncementBannerProps = AnnouncementBannerProps & {
	/**
	 * The key to use for the localStorage item.
	 */
	storageKey: string;
	/**
	 * The callback to call when the banner is dismissed.
	 */
	onDismiss?: () => void;
};

function isDismissed(storageKey: string): boolean {
	return getLocalStorage(storageKey) === 'true';
}

/**
 * Announcement banner that persists its dismiss state in localStorage.
 * Once dismissed, the banner stays hidden until the storage key changes.
 */
const PersistedAnnouncementBanner = forwardRef<HTMLDivElement, PersistedAnnouncementBannerProps>(
	({ storageKey, onDismiss, ...props }, ref) => {
		const [visible, setVisible] = useState(() => !isDismissed(storageKey));

		const handleClose = useCallback(() => {
			setLocalStorage(storageKey, 'true');
			setVisible(false);
			onDismiss?.();
		}, [storageKey, onDismiss]);

		if (!visible) {
			return null;
		}

		return <AnnouncementBanner ref={ref} {...props} onClose={handleClose} />;
	}
);
PersistedAnnouncementBanner.displayName = 'PersistedAnnouncementBanner';

export { PersistedAnnouncementBanner };
