import { PersistedAnnouncementBanner } from 'components/ui/announcement-banner';

import styles from './NoAuthBanner.module.scss';

import type { JSX } from 'react';

export function NoAuthBanner(): JSX.Element {
	return (
		<PersistedAnnouncementBanner
			type="warning"
			storageKey="no-auth-banner-v1"
			testId="no-auth-banner"
			className={styles.banner}
		>
			Impersonation mode: authentication is disabled. Anyone with access to this
			instance has admin privileges.{' '}
			<a
				href="https://o11y.io/docs/manage/administrator-guide/configuration/impersonation-mode/"
				target="_blank"
				rel="noreferrer"
			>
				Learn more
			</a>
		</PersistedAnnouncementBanner>
	);
}

export default NoAuthBanner;
