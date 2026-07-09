import { sentryVitePlugin } from '@sentry/vite-plugin';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';
import { visualizer } from 'rollup-plugin-visualizer';
import type { Plugin, TransformResult, UserConfig } from 'vite';
import { defineConfig, loadEnv } from 'vite';
import vitePluginChecker from 'vite-plugin-checker';
import viteCompression from 'vite-plugin-compression';
import { createHtmlPlugin } from 'vite-plugin-html';
import { ViteImageOptimizer } from 'vite-plugin-image-optimizer';
import tsconfigPaths from 'vite-tsconfig-paths';

// The `[[.BaseHref]]` / `[[.Settings]]` placeholders in index.html were meant to be
// filled by the o11y Go backend at serve time. o11y now runs headless and the SPA is
// served by hanzoai/static (which does no templating), so we resolve them at build
// time — and in the dev server, where the backend is likewise not involved. Both
// paths produce identical placeholder-free HTML. Set VITE_BASE_PATH to serve under a
// URL prefix (e.g. /o11y/); defaults to '/'.
function baseHrefPlugin(basePath: string): Plugin {
	return {
		name: 'base-href',
		transformIndexHtml(html): string {
			return html.replaceAll('[[.BaseHref]]', basePath);
		},
	};
}

function bootSettingsPlugin(env: Record<string, string>): Plugin {
	return {
		name: 'boot-settings',
		transformIndexHtml(html): string {
			const settings = {
				posthog: { enabled: env.VITE_POSTHOG_ENABLED !== 'false' },
				appcues: { enabled: env.VITE_APPCUES_ENABLED !== 'false' },
				sentry: { enabled: env.VITE_SENTRY_ENABLED !== 'false' },
				pylon: { enabled: env.VITE_PYLON_ENABLED !== 'false' },
			};
			return html.replaceAll('[[.Settings]]', JSON.stringify(settings));
		},
	};
}

function rawMarkdownPlugin(): Plugin {
	return {
		name: 'raw-markdown',
		transform(code, id): TransformResult | undefined {
			if (!id.endsWith('.md')) {
				return undefined;
			}
			return {
				code: `export default ${JSON.stringify(code)};`,
				map: null,
			};
		},
	};
}

export default defineConfig(({ mode }): UserConfig => {
	const env = loadEnv(mode, process.cwd(), '');
	// Base path for serving the app (e.g., '/o11y/'). Defaults to '/'.
	const basePath = env.VITE_BASE_PATH || '/';

	const plugins = [
		tsconfigPaths(),
		rawMarkdownPlugin(),
		baseHrefPlugin(basePath),
		bootSettingsPlugin(env),
		react(),
		createHtmlPlugin({
			inject: {
				data: {
					PYLON_APP_ID: env.VITE_PYLON_APP_ID || '',
					APPCUES_APP_ID: env.VITE_APPCUES_APP_ID || '',
				},
			},
		}),
		vitePluginChecker({
			typescript: true,
			// this doubles the build time
			// disabled to use Biome/tsgo (in the future) as alternative
			enableBuild: false,
		}),
	];

	if (env.VITE_SENTRY_AUTH_TOKEN) {
		plugins.push(
			sentryVitePlugin({
				authToken: env.VITE_SENTRY_AUTH_TOKEN,
				org: env.VITE_SENTRY_ORG,
				project: env.VITE_SENTRY_PROJECT_ID,
			}),
		);
	}

	if (env.BUNDLE_ANALYSER === 'true') {
		plugins.push(
			visualizer({
				open: true,
				gzipSize: true,
				brotliSize: true,
			}),
		);
	}

	if (mode === 'production') {
		plugins.push(
			ViteImageOptimizer({
				jpeg: { quality: 80 },
				jpg: { quality: 80 },
			}),
		);
		plugins.push(viteCompression());
	}

	return {
		plugins,
		resolve: {
			alias: {
				// The @hanzo/ui barrel eagerly imports these Next-only modules at the
				// top level. o11y is not a Next app and uses none of them, but a browser
				// cannot resolve bare `next-themes` / `next/*` specifiers — externalizing
				// them leaves bare imports that crash the SPA at load (blank page). Alias
				// to inert browser stubs so the bundle stays self-contained.
				'next-themes': resolve(__dirname, './stubs/next-themes.ts'),
				'next/image': resolve(__dirname, './stubs/next-image.ts'),
				'next/link': resolve(__dirname, './stubs/next-link.ts'),
				'@': resolve(__dirname, './src'),
				utils: resolve(__dirname, './src/utils'),
				types: resolve(__dirname, './src/types'),
				constants: resolve(__dirname, './src/constants'),
				parser: resolve(__dirname, './src/parser'),
				providers: resolve(__dirname, './src/providers'),
				lib: resolve(__dirname, './src/lib'),
			},
		},
		css: {
			preprocessorOptions: {
				less: {
					javascriptEnabled: true,
				},
			},
			modules: {
				localsConvention: 'camelCaseOnly',
			},
		},
		define: {
			// TODO: Remove this in favor of import.meta.env
			'process.env.NODE_ENV': JSON.stringify(mode),
			'process.env.FRONTEND_API_ENDPOINT': JSON.stringify(
				env.VITE_FRONTEND_API_ENDPOINT,
			),
			'process.env.WEBSOCKET_API_ENDPOINT': JSON.stringify(
				env.VITE_WEBSOCKET_API_ENDPOINT,
			),
			'process.env.PYLON_APP_ID': JSON.stringify(env.VITE_PYLON_APP_ID),
			'process.env.PYLON_IDENTITY_SECRET': JSON.stringify(
				env.VITE_PYLON_IDENTITY_SECRET,
			),
			'process.env.APPCUES_APP_ID': JSON.stringify(env.VITE_APPCUES_APP_ID),
			'process.env.POSTHOG_KEY': JSON.stringify(env.VITE_POSTHOG_KEY),
			'process.env.SENTRY_ORG': JSON.stringify(env.VITE_SENTRY_ORG),
			'process.env.SENTRY_PROJECT_ID': JSON.stringify(env.VITE_SENTRY_PROJECT_ID),
			'process.env.SENTRY_DSN': JSON.stringify(env.VITE_SENTRY_DSN),
			'process.env.TUNNEL_URL': JSON.stringify(env.VITE_TUNNEL_URL),
			'process.env.TUNNEL_DOMAIN': JSON.stringify(env.VITE_TUNNEL_DOMAIN),
			'process.env.DOCS_BASE_URL': JSON.stringify(env.VITE_DOCS_BASE_URL),
		},
		// In production, use relative paths so assets work with any base path injected by the backend.
		// In dev, use the configured base path for proper HMR and routing.
		base: mode === 'production' ? './' : basePath,
		build: {
			sourcemap: true,
			outDir: 'build',
			cssMinify: 'esbuild',
			rollupOptions: {
				// @hanzo/ui eagerly imports all optional peer deps at the module
				// level. These heavy web3/viz packages are used by @hanzo/ui subpaths
				// that o11y never imports, so they tree-shake out entirely (0 refs in
				// the built bundle) — externalizing keeps the bundler from erroring on
				// missing resolution. (next-themes/next/* are NOT here: they survive
				// tree-shaking into o11y's used graph, so they are aliased to browser
				// stubs above instead — a bare import of them would crash the SPA.)
				external: [
					'chrono-node',
					'mermaid',
					'recharts',
					'sql.js',
					'react-qrcode-logo',
					'@rainbow-me/rainbowkit',
					'wagmi',
					/^@tanstack\/react-query/,
					'@modelcontextprotocol/sdk',
					'@hanzo/docs-core',
					/^@o11yhq\//,
				],
			},
		},
		server: {
			open: true,
			port: 3301,
			host: true,
		},
		preview: {
			port: 3301,
		},
	};
});
