import {
	createContext,
	useContext,
	useEffect,
	useState,
	ReactNode,
} from 'react';

export interface TenantBranding {
	/** Display name / wordmark text */
	name: string;
	/** Optional logo URL — if empty, wordmark text is rendered instead */
	logo: string;
	/** Favicon URL — falls back to a generated one from primaryColor */
	favicon: string;
	/** Brand accent color (CSS value) — monochrome (#ffffff) by default */
	primaryColor: string;
	/** Organization slug for API scoping */
	orgSlug: string;
	/** OIDC issuer URL */
	issuerUrl: string;
	/** OIDC client ID */
	clientId: string;
	/** Product suffix shown after brand name (e.g. "O11y", "Sentry") */
	productName: string;
}

const DEFAULT_TENANT: TenantBranding = {
	name: 'O11y',
	logo: '',
	favicon: '',
	primaryColor: '#ffffff',
	orgSlug: '',
	issuerUrl: '',
	clientId: '',
	productName: 'O11y',
};

const TenantContext = createContext<TenantBranding>(DEFAULT_TENANT);

export function TenantProvider({ children }: { children: ReactNode }): JSX.Element {
	const [tenant, setTenant] = useState<TenantBranding>(DEFAULT_TENANT);

	useEffect(() => {
		fetch('/api/v1/tenant')
			.then((res) => {
				if (!res.ok) throw new Error(`${res.status}`);
				return res.json();
			})
			.then((data: Partial<TenantBranding>) => {
				const merged: TenantBranding = { ...DEFAULT_TENANT, ...data };
				setTenant(merged);

				// Apply tenant accent color as CSS custom property
				if (merged.primaryColor) {
					document.documentElement.style.setProperty(
						'--tenant-primary-color',
						merged.primaryColor,
					);
				}
				document.documentElement.style.setProperty(
					'--tenant-name',
					`"${merged.name}"`,
				);

				// Page title: "Brand Product" or just "Product"
				const title = merged.name !== merged.productName
					? `${merged.name} ${merged.productName}`
					: merged.productName;
				document.title = title;

				// Update favicon if provided
				if (merged.favicon) {
					const link =
						document.querySelector<HTMLLinkElement>("link[rel='icon']") ||
						document.createElement('link');
					link.rel = 'icon';
					link.href = merged.favicon;
					document.head.appendChild(link);
				}
			})
			.catch(() => {
				// Silently fall back to defaults — app works without tenant API
			});
	}, []);

	return (
		<TenantContext.Provider value={tenant}>{children}</TenantContext.Provider>
	);
}

export function useTenant(): TenantBranding {
	return useContext(TenantContext);
}

export default TenantContext;
