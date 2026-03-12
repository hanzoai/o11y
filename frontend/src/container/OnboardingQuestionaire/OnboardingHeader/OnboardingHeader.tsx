import './OnboardingHeader.styles.scss';

export function OnboardingHeader(): JSX.Element {
	return (
		<div className="header-container">
			<div className="logo-container">
				<img src="/Logos/hanzo-icon.svg" alt="Hanzo" />
				<span className="logo-text">Hanzo O11y</span>
			</div>
		</div>
	);
}
