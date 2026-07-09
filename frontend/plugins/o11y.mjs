/**
 * Oxlint custom rules plugin for O11y.
 *
 * This plugin aggregates all custom O11y linting rules.
 * Individual rules are defined in the ./rules directory.
 */

import noZustandGetStateInHooks from './rules/no-zustand-getstate-in-hooks.mjs';
import noNavigatorClipboard from './rules/no-navigator-clipboard.mjs';
import noUnsupportedAssetPattern from './rules/no-unsupported-asset-pattern.mjs';
import noRawAbsolutePath from './rules/no-raw-absolute-path.mjs';
import noAntdComponents from './rules/no-antd-components.mjs';

export default {
	meta: {
		name: 'o11y',
	},
	rules: {
		'no-zustand-getstate-in-hooks': noZustandGetStateInHooks,
		'no-navigator-clipboard': noNavigatorClipboard,
		'no-unsupported-asset-pattern': noUnsupportedAssetPattern,
		'no-raw-absolute-path': noRawAbsolutePath,
		'no-antd-components': noAntdComponents,
	},
};
