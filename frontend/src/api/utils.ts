import deleteLocalStorageKey from 'api/browser/localstorage/remove';
import { LOCALSTORAGE } from 'constants/localStorage';

// GUARD_LOGOUT is the edge's sign-out, on this same origin, and it is a fixed
// contract exactly as the identity headers are: the hop that put a session in
// front of this app (admin-guard) is the only thing that can end one.
//
// This used to DELETE /v1/o11y/sessions and push the SPA at its own /login. Both
// halves were wrong once identity moved to the edge: o11y mints no session to
// delete, and /login re-entered an app the browser still had a valid cookie for
// — clicking "Sign out" cleared some localStorage keys and put you straight back
// where you were. Clearing local state is housekeeping, not signing out.
const GUARD_LOGOUT = '/__guard/logout';

export const Logout = (): void => {
	deleteLocalStorageKey(LOCALSTORAGE.IS_LOGGED_IN);
	deleteLocalStorageKey(LOCALSTORAGE.IS_IDENTIFIED_USER);
	deleteLocalStorageKey(LOCALSTORAGE.LOGGED_IN_USER_EMAIL);
	deleteLocalStorageKey(LOCALSTORAGE.LOGGED_IN_USER_NAME);
	deleteLocalStorageKey(LOCALSTORAGE.CHAT_SUPPORT);
	deleteLocalStorageKey(LOCALSTORAGE.USER_ID);
	deleteLocalStorageKey(LOCALSTORAGE.QUICK_FILTERS_SETTINGS_ANNOUNCEMENT);
	window.dispatchEvent(new CustomEvent('LOGOUT'));
	window.location.assign(GUARD_LOGOUT);
};

export const UnderscoreToDotMap: Record<string, string> = {
	k8s_cluster_name: 'k8s.cluster.name',
	k8s_cluster_uid: 'k8s.cluster.uid',
	k8s_namespace_name: 'k8s.namespace.name',
	k8s_node_name: 'k8s.node.name',
	k8s_node_uid: 'k8s.node.uid',
	k8s_pod_name: 'k8s.pod.name',
	k8s_pod_uid: 'k8s.pod.uid',
	k8s_deployment_name: 'k8s.deployment.name',
	k8s_daemonset_name: 'k8s.daemonset.name',
	k8s_statefulset_name: 'k8s.statefulset.name',
	k8s_cronjob_name: 'k8s.cronjob.name',
	k8s_job_name: 'k8s.job.name',
	k8s_persistentvolumeclaim_name: 'k8s.persistentvolumeclaim.name',
};
