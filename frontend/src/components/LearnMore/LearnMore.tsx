import { Color } from 'constants/designTokens';
import { Button } from 'antd';
import { ArrowUpRight } from 'components/ui/icons';
import { openInNewTab } from 'utils/navigation';

import './LearnMore.styles.scss';

import type { JSX } from 'react';

type LearnMoreProps = {
	text?: string;
	url?: string;
	onClick?: () => void;
};

function LearnMore({
	text = 'Learn more',
	url = '',
	onClick = (): void => {},
}: LearnMoreProps): JSX.Element {
	const handleClick = (): void => {
		onClick?.();
		if (url) {
			openInNewTab(url);
		}
	};
	return (
		<Button type="link" className="learn-more" onClick={handleClick}>
			<div className="learn-more__text">{text}</div>
			<ArrowUpRight size={16} color={Color.BG_ROBIN_400} />
		</Button>
	);
}

export default LearnMore;
