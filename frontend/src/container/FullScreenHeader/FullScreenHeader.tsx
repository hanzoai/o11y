import history from 'lib/history';

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
				<img src="/Logos/hanzo-icon.svg" alt="Hanzo" />

				<div className="brand-logo-name">Hanzo</div>
			</div>
		</div>
	);
}

FullScreenHeader.defaultProps = {
	overrideRoute: '/',
};
