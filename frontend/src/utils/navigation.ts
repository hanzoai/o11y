import { withBasePath } from 'utils/basePath';

// `noopener` is required: window.open (unlike <a target="_blank">) does NOT imply
// it, so without it the opened tab can reach back through window.opener and
// navigate us — reverse tabnabbing. Safe for every caller: the return is void, so
// nobody can be relying on the WindowProxy that noopener suppresses.
export const openInNewTab = (path: string): void => {
	window.open(withBasePath(path), '_blank', 'noopener');
};
