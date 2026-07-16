import AlertRuleProvider from 'providers/Alert';

import AlertDetails from './AlertDetails';

import type { JSX } from 'react';

function AlertDetailsPage(): JSX.Element {
	return (
		<AlertRuleProvider>
			<AlertDetails />
		</AlertRuleProvider>
	);
}

export default AlertDetailsPage;
