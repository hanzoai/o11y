import type { TypesUserDTO } from 'api/generated/services/sigNoz.schemas';
import userEvent from '@testing-library/user-event';
import { rest, server } from 'mocks-server/server';
import { fireEvent, render, screen } from 'tests/test-utils';

import MembersSettings from '../MembersSettings';

jest.mock('@hanzo/ui', () => ({
	toast: {
		success: jest.fn(),
		error: jest.fn(),
	},
}));

const USERS_ENDPOINT = '*/api/v2/users';

const mockUsers: TypesUserDTO[] = [
	{
		id: 'user-1',
		displayName: 'Alice Smith',
		email: 'alice@o11y.hanzo.ai',
		role: 'ADMIN',
		createdAt: 1700000000,
		organization: 'TestOrg',
		orgId: 'org-1',
	},
	{
		id: 'user-2',
		displayName: 'Bob Jones',
		email: 'bob@o11y.hanzo.ai',
		role: 'VIEWER',
		createdAt: 1700000001,
		organization: 'TestOrg',
		orgId: 'org-1',
	},
	{
		id: 'inv-1',
		email: 'charlie@o11y.hanzo.ai',
		name: 'Charlie',
		role: 'EDITOR',
		createdAt: 1700000002,
		token: 'tok-abc',
	},
];

describe('MembersSettings (integration)', () => {
	beforeEach(() => {
		jest.clearAllMocks();
		server.use(
			rest.get(USERS_ENDPOINT, (_, res, ctx) =>
				res(ctx.status(200), ctx.json({ data: mockUsers })),
			),
		);
	});

	afterEach(() => {
		server.resetHandlers();
	});

	it('loads and displays active users, pending invites, and deleted members', async () => {
		render(<MembersSettings />);

		await screen.findByText('Alice Smith');
		expect(screen.getByText('Bob Jones')).toBeInTheDocument();
		expect(screen.getByText('charlie@o11y.hanzo.ai')).toBeInTheDocument();
		expect(screen.getAllByText('ACTIVE')).toHaveLength(2);
		expect(screen.getByText('INVITED')).toBeInTheDocument();
		expect(screen.getByText('DELETED')).toBeInTheDocument();
	});

	it('filters to pending invites via the filter dropdown', async () => {
		const user = userEvent.setup({ pointerEventsCheck: 0 });
		render(<MembersSettings />);

		await screen.findByText('Alice Smith');

		await user.click(screen.getByRole('button', { name: /all members/i }));

		const pendingOption = await screen.findByText(/pending invites/i);
		await user.click(pendingOption);

		await screen.findByText('charlie@o11y.hanzo.ai');
		expect(screen.queryByText('Alice Smith')).not.toBeInTheDocument();
	});

	it('filters members by name using the search input', async () => {
		render(<MembersSettings />);

		await screen.findByText('Alice Smith');

		fireEvent.change(screen.getByPlaceholderText(/Search by name or email/i), {
			target: { value: 'bob' },
		});

		await screen.findByText('Bob Jones');
		expect(screen.queryByText('Alice Smith')).not.toBeInTheDocument();
		expect(screen.queryByText('charlie@o11y.hanzo.ai')).not.toBeInTheDocument();
	});

	it('opens EditMemberDrawer when an active member row is clicked', async () => {
		render(<MembersSettings />);

		fireEvent.click(await screen.findByText('Alice Smith'));

		await screen.findByText('Member Details');
	});

	it('opens EditMemberDrawer when a deleted member row is clicked', async () => {
		render(<MembersSettings />);

		fireEvent.click(await screen.findByText('Dave Deleted'));

		expect(await screen.findAllByPlaceholderText('john@o11y.hanzo.ai')).toHaveLength(
			3,
		);
	});
});
