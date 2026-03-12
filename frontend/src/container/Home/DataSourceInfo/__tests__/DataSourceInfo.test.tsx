import { GetHosts200 } from 'api/generated/services/observe.schemas';
import { rest, server } from 'mocks-server/server';
import { render, screen } from 'tests/test-utils';

import DataSourceInfo from '../DataSourceInfo';

const ZEUS_HOSTS_ENDPOINT = '*/api/v2/zeus/hosts';

const mockHostsResponse: GetHosts200 = {
	status: 'success',
	data: {
		name: 'accepted-starfish',
		state: 'HEALTHY',
		tier: 'PREMIUM',
		hosts: [
			{
				name: 'accepted-starfish',
				is_default: true,
				url: 'https://accepted-starfish.test.cloud',
			},
			{
				name: 'custom-host',
				is_default: false,
				url: 'https://custom-host.test.cloud',
			},
		],
	},
};

describe('DataSourceInfo', () => {
	afterEach(() => server.resetHandlers());

	it('renders the active workspace URL with protocol stripped', async () => {
		server.use(
			rest.get(ZEUS_HOSTS_ENDPOINT, (_, res, ctx) =>
				res(ctx.status(200), ctx.json(mockHostsResponse)),
			),
		);

		render(<DataSourceInfo dataSentToHanzoO11y={false} isLoading={false} />);

		await screen.findByText(/custom-host\.test\.cloud/i);
	});

	it('does not render workspace URL when GET /zeus/hosts fails', async () => {
		server.use(
			rest.get(ZEUS_HOSTS_ENDPOINT, (_, res, ctx) =>
				res(ctx.status(500), ctx.json({})),
			),
		);

		render(<DataSourceInfo dataSentToHanzoO11y={false} isLoading={false} />);

		await screen.findByText(/Your workspace is ready/i);
		expect(screen.queryByText(/o11y\.cloud/i)).not.toBeInTheDocument();
	});

	it('renders active workspace URL in the data-received view when telemetry is flowing', async () => {
		server.use(
			rest.get(ZEUS_HOSTS_ENDPOINT, (_, res, ctx) =>
				res(ctx.status(200), ctx.json(mockHostsResponse)),
			),
		);

		render(<DataSourceInfo dataSentToHanzoO11y={true} isLoading={false} />);

		await screen.findByText(/custom-host\.test\.cloud/i);
	});
});
