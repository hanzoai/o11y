import { ArrowRight } from 'components/ui/icons';
import { Typography } from 'components/ui/typography';
import { openInNewTab } from 'utils/navigation';

import styles from './AlertsEmptyState.module.scss';

import type { JSX } from 'react';

interface AlertInfoCardProps {
	header: string;
	subheader: string;
	link: string;
	onClick: () => void;
}

function AlertInfoCard({
	header,
	subheader,
	link,
	onClick,
}: AlertInfoCardProps): JSX.Element {
	return (
		<div
			className={styles.alertInfoCard}
			onClick={(): void => {
				onClick();
				openInNewTab(link);
			}}
		>
			<div className={styles.alertCardText}>
				<Typography.Text className={styles.alertCardTextHeader}>
					{header}
				</Typography.Text>
				<Typography.Text className={styles.alertCardTextSubheader}>
					{subheader}
				</Typography.Text>
			</div>
			<ArrowRight size="md" />
		</div>
	);
}

export default AlertInfoCard;
