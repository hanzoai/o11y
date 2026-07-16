import { ReactNode, type JSX } from 'react';

import { StyledAlert } from './styles';

interface MessageTipProps {
	show?: boolean;
	message: ReactNode | string;
	action: ReactNode | undefined;
}

function MessageTip({
	show = false,
	message,
	action,
}: MessageTipProps): JSX.Element | null {
	if (!show) {
		return null;
	}

	return (
		<StyledAlert showIcon description={message} type="info" action={action} />
	);
}

export default MessageTip;
