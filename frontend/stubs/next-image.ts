// Browser stub for `next/image`, eagerly imported by the @hanzo/ui barrel
// (`import Image from 'next/image'`). o11y is not a Next app; this degrades to a
// plain <img>. Must resolve to a real module (aliased in vite.config.ts), never be
// externalized — a bare `next/image` specifier is unresolvable in the browser and
// crashes the SPA at load.
import { createElement, forwardRef } from 'react';
import type { ImgHTMLAttributes, Ref } from 'react';

type ImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, 'src'> & {
	src?: string | { src?: string };
	alt?: string;
};

const Image = forwardRef(function Image(
	{ src, alt = '', ...rest }: ImageProps,
	ref: Ref<HTMLImageElement>,
) {
	const resolvedSrc = typeof src === 'string' ? src : src?.src;
	return createElement('img', { ref, src: resolvedSrc, alt, ...rest });
});

export default Image;
