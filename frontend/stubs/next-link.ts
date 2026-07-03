// Browser stub for `next/link`, eagerly imported by the @hanzo/ui barrel
// (`import Link from 'next/link'`). o11y routes with react-router, not Next; this
// degrades to a plain <a>. Must resolve to a real module (aliased in vite.config.ts),
// never be externalized — a bare `next/link` specifier is unresolvable in the browser
// and crashes the SPA at load.
import { createElement, forwardRef } from 'react';
import type { AnchorHTMLAttributes, Ref } from 'react';

type LinkProps = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
	href?: string | { pathname?: string };
};

const Link = forwardRef(function Link(
	{ href, children, ...rest }: LinkProps,
	ref: Ref<HTMLAnchorElement>,
) {
	const resolvedHref = typeof href === 'string' ? href : href?.pathname ?? '#';
	return createElement('a', { ref, href: resolvedHref, ...rest }, children);
});

export default Link;
