import getLocalStorageApi from 'api/browser/localstorage/get';
import setLocalStorageApi from 'api/browser/localstorage/set';
import { LOCALSTORAGE } from 'constants/localStorage';
import { ENVIRONMENT } from 'constants/env';

export function getO11yInstanceUrl(): string {
	const fromStorage = getLocalStorageApi(
		LOCALSTORAGE.ACTIVE_O11Y_INSTANCE_URL,
	);

	if (typeof fromStorage === 'string' && fromStorage.trim().length > 0) {
		return fromStorage;
	}

	return ENVIRONMENT.baseURL;
}

export function setO11yInstanceUrl(url: string | null | undefined): void {
	const next = (url ?? '').trim();

	if (!next) {
		return;
	}

	setLocalStorageApi(LOCALSTORAGE.ACTIVE_O11Y_INSTANCE_URL, next);
}
