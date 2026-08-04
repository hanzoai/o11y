import {
	Children,
	cloneElement,
	forwardRef,
	isValidElement,
	type CSSProperties,
	type HTMLAttributes,
	type ReactElement,
	type Ref,
} from 'react';

/**
 * `Slot` renders its own props onto its single child instead of onto a wrapper
 * element — the `asChild` escape hatch every styled component here offers.
 *
 * Composition rules, in the order a reader needs them:
 *  - event handlers compose (ours first, then the child's — the child wins the
 *    right to `preventDefault` on top of us);
 *  - `className` concatenates, `style` merges with the child's on top, so a call
 *    site can always override;
 *  - every other prop is a plain override by the child, because the child is the
 *    more specific declaration;
 *  - refs merge, so `<Button asChild ref={r}><a ref={mine}/></Button>` reaches both.
 */

type AnyProps = Record<string, unknown>;

const isHandler = (
	key: string,
	value: unknown,
): value is (...a: never[]) => void =>
	/^on[A-Z]/.test(key) && typeof value === 'function';

const composeRefs =
	<T,>(...refs: (Ref<T> | undefined)[]) =>
	(node: T | null): void => {
		refs.forEach((ref) => {
			if (typeof ref === 'function') {
				ref(node);
			} else if (ref) {
				(ref as { current: T | null }).current = node;
			}
		});
	};

function merge(slotProps: AnyProps, childProps: AnyProps): AnyProps {
	const merged: AnyProps = { ...slotProps, ...childProps };

	Object.keys(slotProps).forEach((key) => {
		const ours = slotProps[key];
		const theirs = childProps[key];

		if (isHandler(key, ours)) {
			merged[key] = isHandler(key, theirs)
				? (...args: never[]): void => {
						ours(...args);
						theirs(...args);
					}
				: ours;
		} else if (key === 'style') {
			merged.style = {
				...(ours as CSSProperties),
				...(theirs as CSSProperties),
			};
		} else if (key === 'className') {
			merged.className = [ours, theirs].filter(Boolean).join(' ');
		}
	});

	return merged;
}

export interface SlotProps extends HTMLAttributes<HTMLElement> {
	children?: React.ReactNode;
}

export const Slot = forwardRef<HTMLElement, SlotProps>(
	({ children, ...slotProps }, ref) => {
		const child = Children.only(children);
		if (!isValidElement(child)) {
			return null;
		}

		const element = child as ReactElement<AnyProps>;
		// React 19 carries ref as a regular prop. Reading element.ref instead would
		// log a removal warning on every render — and find nothing.
		const childRef = element.props.ref as Ref<HTMLElement> | undefined;

		return cloneElement(element, {
			...merge(slotProps as AnyProps, element.props),
			ref: composeRefs(ref, childRef),
		});
	},
);
Slot.displayName = 'Slot';
