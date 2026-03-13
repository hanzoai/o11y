import BrandMark from 'components/BrandMark';

import './OnboardingHeader.styles.scss';

export function OnboardingHeader(): JSX.Element {
	return (
		<div className="header-container">
			<div className="logo-container">
				<BrandMark size={24} showProduct />
			</div>
		</div>
	);
}
