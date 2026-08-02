/**
 * The one API prefix. Every o11y route lives under `/v1/o11y/` — there is no
 * version segment below it and there never will be a `/v2/`.
 *
 * `apiV2`…`apiV5` are aliases kept only so the existing import sites keep
 * compiling; they resolve to the same string. Delete them (and the matching
 * `ApiV*Instance` axios instances in `./index.ts`) as call sites collapse onto
 * the default export.
 */
const apiV1 = '/v1/o11y/';

export const apiV2 = apiV1;
export const apiV3 = apiV1;
export const apiV4 = apiV1;
export const apiV5 = apiV1;
export const apiAlertManager = `${apiV1}alertmanager/`;

export default apiV1;
