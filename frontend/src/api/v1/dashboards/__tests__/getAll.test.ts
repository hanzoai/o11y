import axios from 'api';

import getAll from '../getAll';

jest.mock('api', () => ({
	get: jest.fn(),
}));

// The body GET /v1/o11y/dashboards answers: the V2 list, a page at a time.
const listed = (id: string, name: string, display?: string): object => ({
	id,
	createdAt: '2026-09-01T00:00:00Z',
	updatedAt: '2026-09-02T00:00:00Z',
	createdBy: 'z@hanzo.ai',
	updatedBy: 'z@hanzo.ai',
	orgId: 'org',
	locked: true,
	source: 'user',
	schemaVersion: 'v6',
	name,
	image: 'data:image/svg+xml;base64,AA==',
	tags: [
		{ key: 'team', value: 'infra' },
		{ key: 'prod', value: 'prod' },
	],
	spec: display ? { display: { name: display, description: 'about it' } } : {},
});

const page = (dashboards: object[], total: number): object => ({
	status: 200,
	data: { status: 'success', data: { dashboards, total, tags: [] } },
});

describe('getAll', () => {
	beforeEach(() => jest.clearAllMocks());

	it('reads the V2 list into the Dashboard the legacy screens render', async () => {
		(axios.get as jest.Mock).mockResolvedValueOnce(
			page([listed('a', 'fallback', 'Checkout'), listed('b', 'Payments')], 2),
		);

		const result = await getAll();

		expect(axios.get).toHaveBeenCalledWith('/dashboards', {
			params: { limit: 200, offset: 0, sort: 'updated_at', order: 'desc' },
		});
		expect(result.httpStatusCode).toBe(200);
		expect(result.data).toStrictEqual([
			{
				id: 'a',
				createdAt: '2026-09-01T00:00:00Z',
				updatedAt: '2026-09-02T00:00:00Z',
				createdBy: 'z@hanzo.ai',
				updatedBy: 'z@hanzo.ai',
				locked: true,
				data: {
					title: 'Checkout',
					description: 'about it',
					tags: ['team:infra', 'prod'],
					image: 'data:image/svg+xml;base64,AA==',
					variables: {},
				},
			},
			expect.objectContaining({
				id: 'b',
				data: expect.objectContaining({
					title: 'Payments',
					description: undefined,
				}),
			}),
		]);
		// The screens sort and slice what they get; it has to be an array.
		expect(Array.isArray(result.data)).toBe(true);
	});

	it('reads every page, not only the first', async () => {
		const first = Array.from({ length: 200 }, (_, i) =>
			listed(`p1-${i}`, `d${i}`),
		);
		(axios.get as jest.Mock)
			.mockResolvedValueOnce(page(first, 201))
			.mockResolvedValueOnce(page([listed('p2-0', 'last')], 201));

		const result = await getAll();

		expect(axios.get).toHaveBeenCalledTimes(2);
		expect((axios.get as jest.Mock).mock.calls[1][1]).toStrictEqual({
			params: { limit: 200, offset: 200, sort: 'updated_at', order: 'desc' },
		});
		expect(result.data).toHaveLength(201);
	});

	it('answers an empty org with an empty list', async () => {
		(axios.get as jest.Mock).mockResolvedValueOnce(page([], 0));

		const result = await getAll();

		expect(result.data).toStrictEqual([]);
	});
});
