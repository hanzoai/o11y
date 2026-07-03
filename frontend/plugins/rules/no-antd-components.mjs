/**
 * Rule: no-antd-components
 *
 * Prevents importing specific components from antd.
 *
 * This rule catches patterns like:
 *   import { Typography } from 'antd'
 *   import { Typography, Button } from 'antd'
 *   import Typography from 'antd/es/typography'
 *   import { Text } from 'antd/es/typography'
 *
 * Add components to BANNED_COMPONENTS to ban them.
 * Key should be PascalCase component name, will match lowercase path too.
 */

const BANNED_COMPONENTS = {
	Typography:
		'Use components/ui/typography Typography instead of antd Typography.',
	Switch: 'Use components/ui/switch Switch instead of antd Switch.',
	Dropdown:
		'Use components/ui DropdownMenuSimple (or the composable DropdownMenu primitives) from components/ui/dropdown-menu instead of antd Dropdown.',
	Badge: 'Use components/ui/badge instead of antd Badge.',
	Radio:
		'Use components/ui/radio-group RadioGroup (dots) or components/ui/toggle-group ToggleGroup (segmented buttons) instead of antd Radio.',
	Progress: 'Use components/ui/progress instead of antd Progress.',
	Avatar: 'Use components/ui/avatar instead of antd Avatar.',
	Divider: 'Use components/ui/divider Divider instead of antd Divider.',
	Tag: 'Use components/ui/badge Bagde instead of antd Tag.',
};

export default {
	create(context) {
		return {
			ImportDeclaration(node) {
				const source = node.source.value;

				// Check direct antd import: import { Typography } from 'antd'
				if (source === 'antd') {
					for (const specifier of node.specifiers) {
						if (specifier.type !== 'ImportSpecifier') {
							continue;
						}

						const importedName = specifier.imported.name;
						const message = BANNED_COMPONENTS[importedName];

						if (message) {
							context.report({
								node: specifier,
								message: `Do not import '${importedName}' from antd. ${message}`,
							});
						}
					}
					return;
				}

				// Check antd/es/<component> import: import Typography from 'antd/es/typography'
				const match = source.match(/^antd\/es\/([^/]+)/);
				if (!match) {
					return;
				}

				const pathComponent = match[1].toLowerCase();

				for (const [componentName, message] of Object.entries(BANNED_COMPONENTS)) {
					if (pathComponent === componentName.toLowerCase()) {
						context.report({
							node,
							message: `Do not import from '${source}'. ${message}`,
						});
						break;
					}
				}
			},
		};
	},
};
