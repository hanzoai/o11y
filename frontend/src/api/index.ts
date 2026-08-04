/* eslint-disable sonarjs/cognitive-complexity */
import { QueryClient } from 'react-query';
import getLocalStorageApi from 'api/browser/localstorage/get';
import axios, {
	AxiosError,
	AxiosResponse,
	InternalAxiosRequestConfig,
} from 'axios';
import { ENVIRONMENT } from 'constants/env';
import { Events } from 'constants/events';
import { LOCALSTORAGE } from 'constants/localStorage';
import { getBasePath } from 'utils/basePath';
import { eventEmitter } from 'utils/getEventEmitter';
import { getIsNoAuthMode } from 'utils/noAuthMode';

import apiV1, { apiAlertManager, apiV2, apiV3, apiV4, apiV5 } from './apiV1';

const RESPONSE_TIMEOUT_THRESHOLD = 5000; // 5 seconds
const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			refetchOnWindowFocus: false,
			retry: false,
		},
	},
});

export const interceptorsResponse = (
	value: AxiosResponse<any>,
): Promise<AxiosResponse<any>> => {
	if ((value.config as any)?.metadata) {
		const duration =
			new Date().getTime() - (value.config as any).metadata.startTime;

		if (duration > RESPONSE_TIMEOUT_THRESHOLD && value.config.url !== '/event') {
			eventEmitter.emit(Events.SLOW_API_WARNING, true, {
				duration,
				url: value.config.url,
				threshold: RESPONSE_TIMEOUT_THRESHOLD,
			});

			console.warn(
				`[API Warning] Request to ${value.config.url} took ${duration}ms`,
			);
		}
	}

	return Promise.resolve(value);
};

export const interceptorsRequestResponse = (
	value: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig => {
	// Attach metadata safely (not sent with the request)
	Object.defineProperty(value, 'metadata', {
		value: { startTime: new Date().getTime() },
		enumerable: false, // Prevents it from being included in the request
	});

	const token = getLocalStorageApi(LOCALSTORAGE.AUTH_TOKEN) || '';

	if (value && value.headers) {
		value.headers.Authorization = token ? `Bearer ${token}` : '';
	}

	return value;
};

// Strips the leading '/' from path and joins with base — idempotent if already prefixed.
// e.g. prependBase('/o11y/', '/v1/o11y/') → '/o11y/v1/o11y/'
function prependBase(base: string, path: string): string {
	return path.startsWith(base) ? path : base + path.slice(1);
}

// Prepends the runtime base path to outgoing requests so API calls work under
// a URL prefix (e.g. /o11y/v1/o11y/…). No-op for root deployments and dev
// (dev baseURL is a full http:// URL, not an absolute path).
export const interceptorsRequestBasePath = (
	value: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig => {
	const basePath = getBasePath();
	if (basePath === '/') {
		return value;
	}

	if (value.baseURL?.startsWith('/')) {
		// Production relative baseURL: '/v1/o11y/' → '/o11y/v1/o11y/'
		value.baseURL = prependBase(basePath, value.baseURL);
	} else if (value.baseURL?.startsWith('http')) {
		// Dev absolute baseURL (VITE_FRONTEND_API_ENDPOINT): 'https://host/v1/o11y/' → 'https://host/o11y/v1/o11y/'
		const url = new URL(value.baseURL);
		url.pathname = prependBase(basePath, url.pathname);
		value.baseURL = url.toString();
	} else if (!value.baseURL && value.url?.startsWith('/')) {
		// Orval-generated client (empty baseURL, path in url): '/v1/o11y/rules' → '/o11y/v1/o11y/rules'
		value.url = prependBase(basePath, value.url);
	}

	return value;
};

// A 401 IS AN ANSWER, not an errand.
//
// This used to be forty lines: on any 401 it exchanged a refresh token for a
// fresh pair through /sessions/rotate, replayed the request, and signed the user
// out when either step failed. Every part of that is gone with the tokens — o11y
// mints no session, so there is no pair to rotate — and what is left is the
// behaviour the app ALREADY had whenever no-auth mode was on: reject, and let
// the caller render the error.
//
// The distinction that makes this correct: identity now lives in a cookie the
// EDGE issued. A 401 from o11y is either an authorization answer (you are signed
// in and may not do this), which no retry can change, or a session the edge has
// stopped vouching for — and only a full navigation back through the guard can
// fix that, which an XHR cannot perform. Logging out on a 401 would take a
// perfectly good session away for the first reason.
export const interceptorRejected = async (
	value: AxiosResponse<any>,
): Promise<AxiosResponse<any>> => Promise.reject(value);

const interceptorRejectedBase = async (
	value: AxiosResponse<any>,
): Promise<AxiosResponse<any>> => Promise.reject(value);

const instance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiV1}`,
});

instance.interceptors.request.use(interceptorsRequestResponse);
instance.interceptors.request.use(interceptorsRequestBasePath);
instance.interceptors.response.use(interceptorsResponse, interceptorRejected);

export const AxiosAlertManagerInstance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiAlertManager}`,
});

export const ApiV2Instance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiV2}`,
});
ApiV2Instance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejected,
);
ApiV2Instance.interceptors.request.use(interceptorsRequestResponse);
ApiV2Instance.interceptors.request.use(interceptorsRequestBasePath);

// axios V3
export const ApiV3Instance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiV3}`,
});

ApiV3Instance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejected,
);
ApiV3Instance.interceptors.request.use(interceptorsRequestResponse);
ApiV3Instance.interceptors.request.use(interceptorsRequestBasePath);
//

// axios V4
export const ApiV4Instance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiV4}`,
});

ApiV4Instance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejected,
);
ApiV4Instance.interceptors.request.use(interceptorsRequestResponse);
ApiV4Instance.interceptors.request.use(interceptorsRequestBasePath);
//

// axios V5
export const ApiV5Instance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiV5}`,
});

ApiV5Instance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejected,
);
ApiV5Instance.interceptors.request.use(interceptorsRequestResponse);
ApiV5Instance.interceptors.request.use(interceptorsRequestBasePath);
//

// axios Base
export const LogEventAxiosInstance = axios.create({
	baseURL: `${ENVIRONMENT.baseURL}${apiV1}`,
});

LogEventAxiosInstance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejectedBase,
);
LogEventAxiosInstance.interceptors.request.use(interceptorsRequestResponse);
LogEventAxiosInstance.interceptors.request.use(interceptorsRequestBasePath);
//

AxiosAlertManagerInstance.interceptors.response.use(
	interceptorsResponse,
	interceptorRejected,
);
AxiosAlertManagerInstance.interceptors.request.use(interceptorsRequestResponse);
AxiosAlertManagerInstance.interceptors.request.use(interceptorsRequestBasePath);

export { apiV1 };
export default instance;
