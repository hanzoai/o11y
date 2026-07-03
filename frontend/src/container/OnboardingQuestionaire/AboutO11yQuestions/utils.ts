/**
 * Deterministically shuffles `items` using `seed`, so the same seed always
 * yields the same ordering. Seeding by app version keeps the option order
 * stable within a release while rotating it across releases.
 */
export function seededShuffle<T>(items: T[], seed: string): T[] {
	// mulberry32 PRNG seeded from a cheap xmur3-style string hash.
	let state = 1779033703 ^ seed.length;
	for (let i = 0; i < seed.length; i += 1) {
		state = Math.imul(state ^ seed.charCodeAt(i), 3432918353);
		state = (state << 13) | (state >>> 19);
	}

	const nextRandom = (): number => {
		state |= 0;
		state = (state + 0x6d2b79f5) | 0;
		let t = Math.imul(state ^ (state >>> 15), 1 | state);
		t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};

	const result = [...items];
	for (let i = result.length - 1; i > 0; i -= 1) {
		const j = Math.floor(nextRandom() * (i + 1));
		[result[i], result[j]] = [result[j], result[i]];
	}
	return result;
}
