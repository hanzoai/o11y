import { expect, test as teardown } from '@playwright/test';

const seederUrl = process.env.O11Y_E2E_SEEDER_URL ?? '';

teardown('clear seeded telemetry', async ({ request }) => {
	expect(seederUrl, 'O11Y_E2E_SEEDER_URL not set').not.toBe('');
	for (const signal of ['metrics', 'traces', 'logs'] as const) {
		const response = await request.delete(`${seederUrl}/telemetry/${signal}`, {
			timeout: 60_000,
		});
		expect(response.ok()).toBeTruthy();
	}
});
