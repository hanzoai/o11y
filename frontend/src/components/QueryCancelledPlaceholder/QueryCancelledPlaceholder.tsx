import { Typography } from 'components/ui/typography';
import eyesEmojiUrl from 'assets/Images/eyesEmoji.svg';

import styles from './QueryCancelledPlaceholder.module.scss';

import type { JSX } from 'react';

interface QueryCancelledPlaceholderProps {
	subText?: string;
}

function QueryCancelledPlaceholder({
	subText = undefined,
}: QueryCancelledPlaceholderProps): JSX.Element {
	return (
		<div className={styles.placeholder}>
			<img className={styles.emoji} src={eyesEmojiUrl} alt="eyes emoji" />
			<Typography className={styles.text}>
				Query cancelled.
				<span className={styles.subText}>
					{' '}
					{subText || 'Click "Run Query" to load data.'}
				</span>
			</Typography>
		</div>
	);
}

export default QueryCancelledPlaceholder;
