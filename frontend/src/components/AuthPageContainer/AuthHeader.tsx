import { useCallback } from 'react';
import { Button } from '@signozhq/button';
import { LifeBuoy } from 'lucide-react';

import './AuthHeader.styles.scss';

function AuthHeader(): JSX.Element {
	const handleGetHelp = useCallback((): void => {
		window.open('https://o11y.hanzo.ai/support/', '_blank');
	}, []);

	return (
		<header className="auth-header">
			<div className="auth-header-logo">
				<img
					src="/Logos/hanzo-icon.svg"
					alt="Hanzo"
					className="auth-header-logo-icon"
				/>
				<span className="auth-header-logo-text">Hanzo Observability</span>
			</div>
			<Button
				className="auth-header-help-button"
				prefixIcon={<LifeBuoy size={12} />}
				onClick={handleGetHelp}
			>
				Get Help
			</Button>
		</header>
	);
}

export default AuthHeader;
