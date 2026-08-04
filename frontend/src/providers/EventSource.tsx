import {
	// eslint-disable-next-line no-restricted-imports
	createContext,
	PropsWithChildren,
	useCallback, // eslint-disable-next-line no-restricted-imports
	useContext,
	useEffect,
	useMemo,
	useRef,
	useState,
	type JSX,
} from 'react';
import { useQueryClient } from 'react-query';
import { apiV3 } from 'api/apiV1';
import getLocalStorageApi from 'api/browser/localstorage/get';
import { Logout } from 'api/utils';
import { ENVIRONMENT } from 'constants/env';
import { LIVE_TAIL_HEARTBEAT_TIMEOUT } from 'constants/liveTail';
import { LOCALSTORAGE } from 'constants/localStorage';
import { EventListener, EventSourcePolyfill } from 'event-source-polyfill';
import { useNotifications } from 'hooks/useNotifications';
import APIError from 'types/api/error';
import { withBasePath } from 'utils/basePath';

interface IEventSourceContext {
	eventSourceInstance: EventSourcePolyfill | null;
	isConnectionOpen: boolean;
	isConnectionLoading: boolean;
	isConnectionError: boolean;
	initialLoading: boolean;
	reconnectDueToError: boolean;
	handleStartOpenConnection: (filterExpression?: string) => void;
	handleCloseConnection: () => void;
	handleSetInitialLoading: (value: boolean) => void;
}

const EventSourceContext = createContext<IEventSourceContext>({
	eventSourceInstance: null,
	isConnectionOpen: false,
	isConnectionLoading: false,
	initialLoading: true,
	isConnectionError: false,
	reconnectDueToError: false,
	handleStartOpenConnection: () => {},
	handleCloseConnection: () => {},
	handleSetInitialLoading: () => {},
});

export function EventSourceProvider({
	children,
}: PropsWithChildren): JSX.Element {
	const [isConnectionOpen, setIsConnectionOpen] = useState<boolean>(false);
	const [isConnectionLoading, setIsConnectionLoading] = useState<boolean>(false);
	const [isConnectionError, setIsConnectionError] = useState<boolean>(false);

	const [reconnectDueToError, setReconnectDueToError] = useState<boolean>(false);

	const [initialLoading, setInitialLoading] = useState<boolean>(true);

	const eventSourceRef = useRef<EventSourcePolyfill | null>(null);

	const { notifications } = useNotifications();
	const queryClient = useQueryClient();

	const handleSetInitialLoading = useCallback((value: boolean) => {
		setInitialLoading(value);
	}, []);

	const handleOpenConnection: EventListener = useCallback(() => {
		setIsConnectionLoading(false);
		setIsConnectionOpen(true);
		setInitialLoading(false);
	}, []);

	// A DROPPED STREAM IS A DROPPED STREAM, not a stale token.
	//
	// This used to rotate the session on every EventSource error and sign the
	// user out when the rotation failed — so a network blip, a proxy timeout or a
	// pod restart logged people out of an app whose session they still held.
	// There is no session here to rotate: identity is a cookie the edge issued,
	// and it outlives the stream. Reconnecting is the whole of the response.
	const handleErrorConnection: EventListener = useCallback(() => {
		setIsConnectionOpen(false);
		setIsConnectionLoading(true);
		setInitialLoading(false);
		setReconnectDueToError(true);
		setIsConnectionError(true);
	}, []);

	const destroyEventSourceSession = useCallback(() => {
		if (!eventSourceRef.current) {
			return;
		}

		eventSourceRef.current.close();
		eventSourceRef.current.removeEventListener('error', handleErrorConnection);
		eventSourceRef.current.removeEventListener('open', handleOpenConnection);
	}, [handleErrorConnection, handleOpenConnection]);

	const handleCloseConnection = useCallback(() => {
		setIsConnectionOpen(false);
		setIsConnectionLoading(false);
		setIsConnectionError(false);

		destroyEventSourceSession();
	}, [destroyEventSourceSession]);

	const handleStartOpenConnection = useCallback(
		(filterExpression?: string): void => {
			const apiPath = `${apiV3}logs/livetail?filter=${encodeURIComponent(
				filterExpression || '',
			)}`;
			const eventSourceUrl = ENVIRONMENT.baseURL
				? `${ENVIRONMENT.baseURL}${apiPath}`
				: withBasePath(apiPath);

			eventSourceRef.current = new EventSourcePolyfill(eventSourceUrl, {
				headers: {
					Authorization: `Bearer ${getLocalStorageApi(LOCALSTORAGE.AUTH_TOKEN)}`,
				},
				heartbeatTimeout: LIVE_TAIL_HEARTBEAT_TIMEOUT,
			});

			setIsConnectionLoading(true);
			setIsConnectionError(false);
			setReconnectDueToError(false);

			eventSourceRef.current.addEventListener('error', handleErrorConnection);
			eventSourceRef.current.addEventListener('open', handleOpenConnection);
		},
		[handleErrorConnection, handleOpenConnection],
	);

	useEffect(
		() => (): void => {
			handleCloseConnection();
		},
		[handleCloseConnection],
	);

	const contextValue: IEventSourceContext = useMemo(
		() => ({
			eventSourceInstance: eventSourceRef.current,
			isConnectionError,
			isConnectionLoading,
			isConnectionOpen,
			initialLoading,
			reconnectDueToError,
			handleStartOpenConnection,
			handleCloseConnection,
			handleSetInitialLoading,
		}),
		[
			isConnectionError,
			isConnectionLoading,
			isConnectionOpen,
			initialLoading,
			reconnectDueToError,
			handleStartOpenConnection,
			handleCloseConnection,
			handleSetInitialLoading,
		],
	);

	return (
		<EventSourceContext.Provider value={contextValue}>
			{children}
		</EventSourceContext.Provider>
	);
}

export const useEventSource = (): IEventSourceContext => {
	const context = useContext(EventSourceContext);

	if (!context) {
		throw new Error('Should be used inside the context');
	}

	return context;
};
