import deleteLocalStorageKey from 'api/browser/localstorage/remove';
import { LOCALSTORAGE } from 'constants/localStorage';

// IAM_LOGOUT is Hanzo IAM's end-session endpoint (end_session_endpoint in
// https://hanzo.id/.well-known/openid-configuration). o11y mints no session of
// its own, so signing out is ending the IAM session; clearing local state is
// housekeeping, not signing out.
const IAM_LOGOUT = 'https://hanzo.id/v1/iam/oauth/logout';

export const Logout = (): void => {
	deleteLocalStorageKey(LOCALSTORAGE.IS_LOGGED_IN);
	deleteLocalStorageKey(LOCALSTORAGE.IS_IDENTIFIED_USER);
	deleteLocalStorageKey(LOCALSTORAGE.LOGGED_IN_USER_EMAIL);
	deleteLocalStorageKey(LOCALSTORAGE.LOGGED_IN_USER_NAME);
	deleteLocalStorageKey(LOCALSTORAGE.CHAT_SUPPORT);
	deleteLocalStorageKey(LOCALSTORAGE.USER_ID);
	deleteLocalStorageKey(LOCALSTORAGE.QUICK_FILTERS_SETTINGS_ANNOUNCEMENT);
	window.dispatchEvent(new CustomEvent('LOGOUT'));
	window.location.assign(IAM_LOGOUT);
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
