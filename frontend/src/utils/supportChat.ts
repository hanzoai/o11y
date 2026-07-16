import { openInNewTab } from 'utils/navigation';

/**
 * The ONE way to open support chat.
 *
 * Replaces the upstream fork's third-party support widget (which loaded an
 * external script and HMAC-hashed user emails for a third party). Support chat
 * is Hanzo Chat — native, ours. Every call site goes through this helper; do not
 * reintroduce a per-component chat integration.
 *
 * Follow-up: embed `@hanzo/chat` in-app (built on @hanzo/ai + @hanzo/gui) so
 * support is a panel rather than a new tab. `@hanzo/ui` is already a dependency.
 */
export const HANZO_CHAT_URL = 'https://hanzo.chat';

export function openSupportChat(): void {
	openInNewTab(HANZO_CHAT_URL);
}
