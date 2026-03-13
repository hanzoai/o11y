import { useCallback } from 'react';
import { Button } from '@hanzo/ui';
import BrandMark from 'components/BrandMark';
import { LifeBuoy } from 'lucide-react';

import './AuthHeader.styles.scss';

function AuthHeader(): JSX.Element {
	const handleGetHelp = useCallback((): void => {
		window.open('/support/', '_blank');
	}, []);

	return (
		<header className="auth-header">
			<div className="auth-header-logo">
				<BrandMark size={20} showProduct />
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
