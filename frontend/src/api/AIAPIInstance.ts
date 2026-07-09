import axios, { InternalAxiosRequestConfig } from 'axios';

import {
	interceptorRejected,
	interceptorsRequestBasePath,
	interceptorsRequestResponse,
	interceptorsResponse,
} from 'api';
import { getO11yInstanceUrl } from 'utils/o11yInstanceUrl';

/** Path-only base for the AI Assistant API. */
export const AI_API_PATH = '/api/v1/assistant';

/** Header that tells the AI backend which O11y instance to query against. */
export const O11Y_URL_HEADER = 'X-O11y-URL';

/**
 * Sets `X-O11y-URL` on every outgoing AI Assistant request. The backend
 * needs the originating O11y instance URL for multi-tenant deployments;
 * when omitted it falls back to its `O11Y_API_URL` env var.
 */
export const interceptorsRequestO11yUrl = (
	value: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig => {
	if (value.headers) {
		value.headers[O11Y_URL_HEADER] = getO11yInstanceUrl();
	}
	return value;
};

/**
 * AI backend URL — sourced from the global config's `ai_assistant_url` field
 * at runtime. `useIsAIAssistantEnabled` keeps this in sync via `setAIBackendUrl`
 * whenever the config response changes; consumers (the axios instance and the
 * SSE fetch path) read it lazily so they always see the current value.
 */
let aiBackendUrl: string | null = null;

export function setAIBackendUrl(url: string | null): void {
	if (aiBackendUrl === url) {
		return;
	}

	aiBackendUrl = url;
	AIAssistantInstance.defaults.baseURL = url ? `${url}${AI_API_PATH}` : '';
}

/**
 * Full base URL for the AI Assistant API (host + path). Throws when the
 * config hasn't yet provided a URL — should never happen in practice
 * because `useIsAIAssistantEnabled` gates every consumer surface.
 */
export function getAIBaseUrl(): string {
	if (!aiBackendUrl) {
		throw new Error('AI assistant URL is not configured.');
	}
	return `${aiBackendUrl}${AI_API_PATH}`;
}

/**
 * Dedicated axios instance for the AI Assistant.
 *
 * Mirrors the request/response interceptor stack of the main O11y axios
 * instance — most importantly `interceptorRejected`, which transparently
 * rotates the access token via `/sessions/rotate` on a 401 and replays the
 * original request. That's why we don't need any AI-specific 401 handling
 * for REST calls: this instance inherits the same flow as the rest of the
 * app for free.
 *
 * Only the SSE stream (`streamEvents`) still needs raw fetch since axios
 * doesn't expose `ReadableStream` — that path keeps its own auth wrapper.
 */
export const AIAssistantInstance = axios.create({});

AIAssistantInstance.interceptors.request.use(interceptorsRequestResponse);
AIAssistantInstance.interceptors.request.use(interceptorsRequestBasePath);
AIAssistantInstance.interceptors.request.use(interceptorsRequestO11yUrl);
AIAssistantInstance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejected,
);
