import ROUTES from './routes';

export const DOCS_BASE_URL = process.env.DOCS_BASE_URL || 'https://o11y.hanzo.ai';

export const WITHOUT_SESSION_PATH = ['/redirect'];

export const AUTH0_REDIRECT_PATH = '/redirect';

export const DEFAULT_AUTH0_APP_REDIRECTION_PATH = ROUTES.APPLICATION;

export const INVITE_MEMBERS_HASH = '#invite-team-members';

export const HANZO_UPGRADE_PLAN_URL =
	'https://upgrade.o11y.hanzo.ai/upgrade-from-app';

export const DASHBOARD_TIME_IN_DURATION = 'refreshInterval';

export const DEFAULT_ENTITY_VERSION = 'v3';
export const ENTITY_VERSION_V4 = 'v4';
export const ENTITY_VERSION_V5 = 'v5';
