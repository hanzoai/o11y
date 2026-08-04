import { RouteTabProps } from 'components/RouteTab/types';
import { TFunction } from 'i18next';
import { ROLES, USER_ROLES } from 'types/roles';

import {
	alertChannels,
	billingSettings,
	createAlertChannels,
	editAlertChannels,
	generalSettings,
	ingestionSettings,
	keyboardShortcuts,
	mcpServerSettings,
	multiIngestionSettings,
	mySettings,
	organizationSettings,
	roleDetails,
	rolesSettings,
	serviceAccountsSettings,
} from './config';

export const getRoutes = (
	userRole: ROLES | null,
	isCurrentOrgSettings: boolean,
	isGatewayEnabled: boolean,
	isWorkspaceBlocked: boolean,
	isCloudUser: boolean,
	isEnterpriseSelfHostedUser: boolean,
	t: TFunction,
): RouteTabProps['routes'] => {
	const settings = [];

	const isAdmin = userRole === USER_ROLES.ADMIN;
	const isEditor = userRole === USER_ROLES.EDITOR;

	if (isWorkspaceBlocked && isAdmin) {
		settings.push(
			...organizationSettings(t),
			...mySettings(t),
			...billingSettings(t),
			...keyboardShortcuts(t),
		);

		return settings;
	}

	settings.push(...generalSettings(t));

	if (isCurrentOrgSettings) {
		settings.push(...organizationSettings(t));
	}

	if (isGatewayEnabled && (isAdmin || isEditor)) {
		settings.push(...multiIngestionSettings(t));
	}

	if (isCloudUser && !isGatewayEnabled) {
		settings.push(...ingestionSettings(t));
	}

	settings.push(...alertChannels(t));

	// Visible to all authenticated users
	settings.push(
		...serviceAccountsSettings(t),
		...rolesSettings(t),
		...roleDetails(t),
	);

	// NO MEMBERS TAB. Listing, inviting, editing and removing members is the Hanzo
	// IAM console's, and o11y no longer serves the routes it would have called.
	// The ROLES tab stays: o11y's roles are its own vocabulary for what may be
	// done to its own dashboards, alerts and views.

	if ((isCloudUser || isEnterpriseSelfHostedUser) && isAdmin) {
		settings.push(...billingSettings(t));
	}

	settings.push(
		...mySettings(t),
		...createAlertChannels(t),
		...editAlertChannels(t),
		...keyboardShortcuts(t),
		...mcpServerSettings(t),
	);

	return settings;
};
