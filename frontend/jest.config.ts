import type { Config } from '@jest/types';

const USE_SAFE_NAVIGATE_MOCK_PATH = '<rootDir>/__mocks__/useSafeNavigate.ts';

const config: Config.InitialOptions = {
	silent: true,
	clearMocks: true,
	coverageDirectory: 'coverage',
	coverageReporters: ['text', 'cobertura', 'html', 'json-summary'],
	collectCoverageFrom: ['src/**/*.{ts,tsx}'],
	moduleFileExtensions: ['ts', 'tsx', 'js', 'json'],
	modulePathIgnorePatterns: ['dist'],
	moduleNameMapper: {
		'\\.(css|less|scss)$': '<rootDir>/__mocks__/cssMock.ts',
		'\\.md$': '<rootDir>/__mocks__/cssMock.ts',
		'^uplot$': '<rootDir>/__mocks__/uplotMock.ts',
		'^hooks/useSafeNavigate$': USE_SAFE_NAVIGATE_MOCK_PATH,
		'^src/hooks/useSafeNavigate$': USE_SAFE_NAVIGATE_MOCK_PATH,
		'^.*/useSafeNavigate$': USE_SAFE_NAVIGATE_MOCK_PATH,
		'^constants/env$': '<rootDir>/__mocks__/env.ts',
		'^src/constants/env$': '<rootDir>/__mocks__/env.ts',
		'^@o11yhq/icons$':
			'<rootDir>/node_modules/@o11yhq/icons/dist/index.esm.js',
		'^react-syntax-highlighter/dist/esm/(.*)$':
			'<rootDir>/node_modules/react-syntax-highlighter/dist/cjs/$1',
		'^@o11yhq/([^/]+)$': '<rootDir>/node_modules/@o11yhq/$1/dist/$1.js',
	},
	extensionsToTreatAsEsm: ['.ts'],
	testMatch: ['<rootDir>/src/**/*?(*.)(test).(ts|js)?(x)'],
	preset: 'ts-jest/presets/js-with-ts-esm',
	transform: {
		'^.+\\.(ts|tsx)?$': [
			'ts-jest',
			{
				useESM: true,
				tsconfig: '<rootDir>/tsconfig.jest.json',
			},
		],
		'^.+\\.(js|jsx)$': 'babel-jest',
	},
	transformIgnorePatterns: [
		'node_modules/(?!(lodash-es|react-dnd|core-dnd|@react-dnd|dnd-core|react-dnd-html5-backend|axios|@o11yhq/design-tokens|@o11yhq/table|@o11yhq/calendar|@o11yhq/input|@o11yhq/popover|@o11yhq/button|@o11yhq/sonner|@o11yhq/*|date-fns|d3-interpolate|d3-color|api|@codemirror|@lezer|@marijn|@grafana|nuqs)/)',
	],
	setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
	testPathIgnorePatterns: ['/node_modules/', '/public/'],
	moduleDirectories: ['node_modules', 'src'],
	testEnvironment: 'jest-environment-jsdom',
	coverageThreshold: {
		global: {
			statements: 80,
			branches: 65,
			functions: 80,
			lines: 80,
		},
	},
};

export default config;
