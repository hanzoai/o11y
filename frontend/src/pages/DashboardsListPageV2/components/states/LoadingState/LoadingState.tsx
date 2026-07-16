import { Skeleton } from 'antd';

import styles from './LoadingState.module.scss';

import type { JSX } from 'react';

function LoadingState(): JSX.Element {
	return (
		<div className={styles.wrapper}>
			<Skeleton.Input active size="large" className={styles.skeleton} />
			<Skeleton.Input active size="large" className={styles.skeleton} />
			<Skeleton.Input active size="large" className={styles.skeleton} />
			<Skeleton.Input active size="large" className={styles.skeleton} />
		</div>
	);
}

export default LoadingState;
