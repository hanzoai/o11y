// Browser stub for `next-themes`, eagerly imported by the @hanzo/ui barrel
// (`import { useTheme } from 'next-themes'`). o11y is not a Next app and drives
// its own theming (index.html boot script + hooks/useDarkMode), so this returns a
// stable dark default. A real browser cannot resolve the bare `next-themes`
// specifier, so it must resolve to this module (aliased in vite.config.ts) instead
// of being externalized — otherwise the SPA crashes at load with a blank page.
import type { ReactNode } from 'react';

export function useTheme(): {
	theme: string;
	setTheme: (t: string) => void;
	resolvedTheme: string;
	systemTheme: string;
	themes: string[];
} {
	return {
		theme: 'dark',
		setTheme: () => undefined,
		resolvedTheme: 'dark',
		systemTheme: 'dark',
		themes: ['light', 'dark'],
	};
}

export function ThemeProvider({ children }: { children?: ReactNode }): ReactNode {
	return children ?? null;
}
