// @hanzo/ui@5.6.0 declares `"types": "./dist/index.d.ts"` in its exports map but
// ships zero declaration files (the package contains no .d.ts). Without this
// ambient module every `@hanzo/ui` import fails type-checking with TS7016
// ("implicitly has an 'any' type"). This keeps the TypeScript program green until
// the upstream package publishes its types; build/runtime resolution is unaffected
// (the JS entry points resolve normally).
declare module '@hanzo/ui';
