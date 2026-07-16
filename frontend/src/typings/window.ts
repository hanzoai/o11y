// eslint-disable-next-line no-restricted-imports
import { compose, Store } from 'redux';
import type { WebSettings } from 'types/generated/webSettings';

declare global {
	interface Window {
		store: Store;
		__REDUX_DEVTOOLS_EXTENSION_COMPOSE__: typeof compose;
		o11yBootData?: { settings: WebSettings | null };
	}
}

export {};
