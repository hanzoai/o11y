import { IntegrationType } from 'container/Integrations/types';

import Header from './Header/Header';
import ServicesTabs from './ServiceTabs/ServicesTabs';

import './CloudIntegration.styles.scss';

import type { JSX } from 'react';

const CloudIntegration = ({ type }: { type: IntegrationType }): JSX.Element => {
	return (
		<div className="cloud-integration-container">
			<Header type={type} />
			<ServicesTabs type={type} />
		</div>
	);
};

export default CloudIntegration;
