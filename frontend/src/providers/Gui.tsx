import { ReactNode, type JSX } from 'react';
import { GuiProvider } from '@hanzo/gui';
import { config } from '@hanzo/ui/gui-config';
import { useIsDarkMode } from 'hooks/useDarkMode';

// The @hanzo/ui components this console renders (Button, Badge, Toaster) are
// @hanzo/gui components, and gui reads its config from module state that
// createGui fills. Nothing here called it, so the first one to render threw
// Err0 and the console fell to "Something went wrong". The config is the
// fleet's — @hanzo/ui/gui-config, the table console and hanzo.ai mount — so a
// Button is the same size here as on every other Hanzo surface. The theme
// follows this app's own dark/light switch.
export function Gui({ children }: { children: ReactNode }): JSX.Element {
	const isDarkMode = useIsDarkMode();

	return (
		<GuiProvider config={config} defaultTheme={isDarkMode ? 'dark' : 'light'}>
			{children}
		</GuiProvider>
	);
}
