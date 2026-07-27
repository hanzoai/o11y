import history from 'lib/history';

export const handleContactSupport = (isCloudUser: boolean): void => {
	if (isCloudUser) {
		history.push('/support');
	} else {
		window.open('https://o11y.io/slack', '_blank');
	}
};
