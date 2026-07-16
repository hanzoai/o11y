// antd 5's static APIs (message.*, notification.*, Modal.confirm) render through
// ReactDOM.render, which React 19 removed — they no-op without this patch, and no
// typecheck catches it. Must be imported before antd is used. Retire it when antd
// reaches a major that speaks React 19 natively.
import '@ant-design/v5-patch-for-react-19';

import { createRoot } from 'react-dom/client';
import { HelmetProvider } from 'react-helmet-async';
import { QueryClient, QueryClientProvider } from 'react-query';
// eslint-disable-next-line no-restricted-imports
import { Provider } from 'react-redux';
import AppRoutes from 'AppRoutes';
import { AxiosError } from 'axios';
import { GlobalTimeStoreAdapter } from 'components/GlobalTimeStoreAdapter/GlobalTimeStoreAdapter';
import { ThemeProvider } from 'hooks/useDarkMode';
import { NuqsAdapter } from 'nuqs/adapters/react';
import { AppProvider } from 'providers/App/App';
import { TenantProvider } from 'providers/Tenant';
import TimezoneProvider from 'providers/Timezone';
import store from 'store';
import APIError from 'types/api/error';

import './ReactI18';

import 'styles.scss';

const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			refetchOnWindowFocus: false,
			retry(failureCount, error): boolean {
				if (
					// in case of manually throwing errors please make sure to send error.response.status
					(error instanceof AxiosError &&
						error.response?.status &&
						error.response?.status >= 400 &&
						error.response?.status <= 499) ||
					(error instanceof APIError &&
						error.getHttpStatusCode() >= 400 &&
						error.getHttpStatusCode() <= 499)
				) {
					return false;
				}
				return failureCount < 2;
			},
		},
	},
});

const container = document.getElementById('root');

if (container) {
	const root = createRoot(container);

	root.render(
		<HelmetProvider>
			<NuqsAdapter>
				<TenantProvider>
					<ThemeProvider>
						<TimezoneProvider>
							<QueryClientProvider client={queryClient}>
								<Provider store={store}>
									<AppProvider>
										<AppRoutes />
									</AppProvider>
								</Provider>
							</QueryClientProvider>
						</TimezoneProvider>
					</ThemeProvider>
				</TenantProvider>
			</NuqsAdapter>
		</HelmetProvider>,
	);
}
