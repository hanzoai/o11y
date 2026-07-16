import { buildPermission } from './utils';

export const IsAdminPermission = buildPermission('assignee', 'role:o11y-admin');
export const IsEditorPermission = buildPermission(
	'assignee',
	'role:o11y-editor',
);
export const IsViewerPermission = buildPermission(
	'assignee',
	'role:o11y-viewer',
);
export const IsAnonymousPermission = buildPermission(
	'assignee',
	'role:o11y-anonymous',
);
