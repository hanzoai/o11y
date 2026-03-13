import inviteUsers from 'api/v1/invite/bulk/create';
import sendInvite from 'api/v1/invite/create';
import { render, screen, userEvent, waitFor } from 'tests/test-utils';

import InviteMembersModal from '../InviteMembersModal';

jest.mock('api/v1/invite/create');
jest.mock('api/v1/invite/bulk/create');
jest.mock('@hanzo/ui', () => ({
	toast: {
		success: jest.fn(),
		error: jest.fn(),
	},
}));

const mockSendInvite = jest.mocked(sendInvite);
const mockInviteUsers = jest.mocked(inviteUsers);

const defaultProps = {
	open: true,
	onClose: jest.fn(),
	onComplete: jest.fn(),
};

describe('InviteMembersModal', () => {
	beforeEach(() => {
		jest.clearAllMocks();
		mockSendInvite.mockResolvedValue({
			httpStatusCode: 200,
			data: { data: 'test', status: 'success' },
		});
		mockInviteUsers.mockResolvedValue({ httpStatusCode: 200, data: null });
	});

	it('renders 3 initial empty rows and disables the submit button', () => {
		render(<InviteMembersModal {...defaultProps} />);

		const emailInputs = screen.getAllByPlaceholderText('john@o11y.hanzo.ai');
		expect(emailInputs).toHaveLength(3);

		expect(
			screen.getByRole('button', { name: /invite team members/i }),
		).toBeDisabled();
	});

	it('adds a row when "Add another" is clicked and removes a row via trash button', async () => {
		const user = userEvent.setup({ pointerEventsCheck: 0 });

		render(<InviteMembersModal {...defaultProps} />);

		await user.click(screen.getByRole('button', { name: /add another/i }));
		expect(screen.getAllByPlaceholderText('john@o11y.hanzo.ai')).toHaveLength(4);

		const removeButtons = screen.getAllByRole('button', { name: /remove row/i });
		await user.click(removeButtons[0]);
		expect(screen.getAllByPlaceholderText('john@o11y.hanzo.ai')).toHaveLength(3);
	});

	describe('validation callout messages', () => {
		it('shows combined message when email is invalid and role is missing', async () => {
			const user = userEvent.setup({ pointerEventsCheck: 0 });

			render(<InviteMembersModal {...defaultProps} />);

			await user.type(
				screen.getAllByPlaceholderText('john@o11y.hanzo.ai')[0],
				'not-an-email',
			);
			await user.click(
				screen.getByRole('button', { name: /invite team members/i }),
			);

			expect(
				await screen.findByText(
					'Please enter valid emails and select roles for team members',
				),
			).toBeInTheDocument();
		});

		it('shows email-only message when email is invalid but role is selected', async () => {
			const user = userEvent.setup({ pointerEventsCheck: 0 });

			render(<InviteMembersModal {...defaultProps} />);

			const emailInputs = screen.getAllByPlaceholderText('john@o11y.hanzo.ai');
			await user.type(emailInputs[0], 'not-an-email');

			await user.click(screen.getAllByText('Select roles')[0]);
			await user.click(await screen.findByText('Viewer'));

			await user.click(
				screen.getByRole('button', { name: /invite team members/i }),
			);

			expect(
				await screen.findByText('Please enter valid emails for team members'),
			).toBeInTheDocument();
		});

		it('shows role-only message when email is valid but role is missing', async () => {
			const user = userEvent.setup({ pointerEventsCheck: 0 });

			render(<InviteMembersModal {...defaultProps} />);

			await user.type(
				screen.getAllByPlaceholderText('john@o11y.hanzo.ai')[0],
				'valid@o11y.hanzo.ai',
			);
			await user.click(
				screen.getByRole('button', { name: /invite team members/i }),
			);

			expect(
				await screen.findByText('Please select roles for team members'),
			).toBeInTheDocument();
		});
	});

	it('uses sendInvite (single) when only one row is filled', async () => {
		const user = userEvent.setup({ pointerEventsCheck: 0 });
		const onComplete = jest.fn();

		render(<InviteMembersModal {...defaultProps} onComplete={onComplete} />);

		const emailInputs = screen.getAllByPlaceholderText('john@o11y.hanzo.ai');
		await user.type(emailInputs[0], 'single@o11y.hanzo.ai');

		const roleSelects = screen.getAllByText('Select roles');
		await user.click(roleSelects[0]);
		await user.click(await screen.findByText('Viewer'));

		await user.click(
			screen.getByRole('button', { name: /invite team members/i }),
		);

		await waitFor(() => {
			expect(mockSendInvite).toHaveBeenCalledWith(
				expect.objectContaining({ email: 'single@o11y.hanzo.ai', role: 'VIEWER' }),
			);
			expect(mockInviteUsers).not.toHaveBeenCalled();
			expect(onComplete).toHaveBeenCalled();
		});
	});

	it('uses inviteUsers (bulk) when multiple rows are filled', async () => {
		const user = userEvent.setup({ pointerEventsCheck: 0 });
		const onComplete = jest.fn();

		render(<InviteMembersModal {...defaultProps} onComplete={onComplete} />);

		const emailInputs = screen.getAllByPlaceholderText('john@o11y.hanzo.ai');

		await user.type(emailInputs[0], 'alice@o11y.hanzo.ai');
		await user.click(screen.getAllByText('Select roles')[0]);
		await user.click(await screen.findByText('Viewer'));

		await user.type(emailInputs[1], 'bob@o11y.hanzo.ai');
		await user.click(screen.getAllByText('Select roles')[0]);
		const editorOptions = await screen.findAllByText('Editor');
		await user.click(editorOptions[editorOptions.length - 1]);

		await user.click(
			screen.getByRole('button', { name: /invite team members/i }),
		);

		await waitFor(() => {
			expect(mockInviteUsers).toHaveBeenCalledWith({
				invites: expect.arrayContaining([
					expect.objectContaining({ email: 'alice@o11y.hanzo.ai', role: 'VIEWER' }),
					expect.objectContaining({ email: 'bob@o11y.hanzo.ai', role: 'EDITOR' }),
				]),
			});
			expect(mockSendInvite).not.toHaveBeenCalled();
			expect(onComplete).toHaveBeenCalled();
		});
	});
});
