import './OnboardingHeader.styles.scss';

export function OnboardingHeader(): JSX.Element {
	return (
		<div className="header-container">
			<div className="logo-container">
				<img src="/Logos/observe-brand-logo.svg" alt="Hanzo O11y" />
				<span className="logo-text">Hanzo O11y</span>
			</div>
		</div>
	);
}
