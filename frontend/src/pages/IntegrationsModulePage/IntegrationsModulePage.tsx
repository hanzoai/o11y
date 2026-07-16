import Integrations from 'container/Integrations/Integrations';

import './IntegrationsModulePage.styles.scss';

import type { JSX } from 'react';

function IntegrationsModulePage(): JSX.Element {
	return (
		<div className="integrations-module-container">
			<Integrations />
		</div>
	);
}

export default IntegrationsModulePage;
