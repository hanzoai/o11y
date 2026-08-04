import type { KeyboardEvent } from 'react';

/**
 * Arrow-key navigation for a group of one-of-N controls (radio group, toggle
 * group, tab list).
 *
 * The set of navigable items is read from the DOM at keypress time rather than
 * kept in a registry: the group element already IS the ordered collection, so a
 * registration protocol would be a second copy of a fact the document holds —
 * and one that drifts whenever items mount conditionally.
 */

const ITEM = '[data-roving-item]';

/** Enabled items of the group that owns `from`, in document order. */
function items(from: HTMLElement): HTMLElement[] {
	const group = from.closest('[data-roving-group]');
	if (!group) {
		return [from];
	}
	return Array.from(group.querySelectorAll<HTMLElement>(ITEM)).filter(
		(el) =>
			!el.hasAttribute('disabled') && el.getAttribute('aria-disabled') !== 'true',
	);
}

const NEXT = new Set(['ArrowDown', 'ArrowRight']);
const PREV = new Set(['ArrowUp', 'ArrowLeft']);

/**
 * Handles ArrowUp/Down/Left/Right, Home and End. `onMove` receives the element
 * focus landed on, so a radio group can select it and a tab list can activate it.
 *
 * `dir: 'rtl'` swaps the horizontal keys only — vertical arrows are unaffected
 * by reading direction.
 */
export function rovingKeyDown(
	event: KeyboardEvent<HTMLElement>,
	onMove?: (el: HTMLElement) => void,
	dir: 'ltr' | 'rtl' = 'ltr',
): void {
	const { key } = event;
	if (!NEXT.has(key) && !PREV.has(key) && key !== 'Home' && key !== 'End') {
		return;
	}

	const current = event.currentTarget;
	const list = items(current);
	if (list.length === 0) {
		return;
	}

	const flip = dir === 'rtl' && (key === 'ArrowLeft' || key === 'ArrowRight');
	const forward = flip ? PREV.has(key) : NEXT.has(key);
	const index = list.indexOf(current);

	let next: HTMLElement;
	if (key === 'Home') {
		next = list[0];
	} else if (key === 'End') {
		next = list[list.length - 1];
	} else {
		next = list[(index + (forward ? 1 : -1) + list.length) % list.length];
	}

	event.preventDefault();
	next.focus();
	onMove?.(next);
}

/**
 * A group with no selection has nothing to put in the tab order, so the group
 * element itself is tabbable and hands focus straight to its first item. Wire
 * this to the group's `onFocus`; it ignores focus that bubbled up from a child.
 */
export function focusFirstItem(event: {
	target: EventTarget | null;
	currentTarget: HTMLElement;
}): void {
	if (event.target !== event.currentTarget) {
		return;
	}
	event.currentTarget
		.querySelector<HTMLElement>(`${ITEM}:not([disabled])`)
		?.focus();
}
