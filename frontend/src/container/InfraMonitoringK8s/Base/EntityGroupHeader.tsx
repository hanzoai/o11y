import { Group } from 'components/ui/icons';

import styles from './EntityGroupHeader.module.scss';

import type { JSX } from 'react';

interface EntityGroupHeaderProps {
	title: string;
	icon?: React.ReactNode;
}

function EntityGroupHeader({
	title,
	icon,
}: EntityGroupHeaderProps): JSX.Element {
	return (
		<div className={styles.entityGroupHeader}>
			{icon || <Group size={14} data-hide-expanded="true" />} {title}
		</div>
	);
}

export default EntityGroupHeader;
