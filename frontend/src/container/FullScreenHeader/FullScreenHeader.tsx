import BrandMark from 'components/BrandMark';
import history from 'lib/history';

import o11yBrandLogoUrl from '@/assets/Logos/o11y-brand-logo.svg';

import './FullScreenHeader.styles.scss';

export default function FullScreenHeader({
	overrideRoute,
}: {
	overrideRoute?: string;
}): React.ReactElement {
	const handleLogoClick = (): void => {
		history.push(overrideRoute || '/');
	};
	return (
		<div className="full-screen-header-container">
			<div className="brand-logo" onClick={handleLogoClick}>
				<BrandMark size={24} />
			</div>
		</div>
	);
}

FullScreenHeader.defaultProps = {
	overrideRoute: '/',
};
