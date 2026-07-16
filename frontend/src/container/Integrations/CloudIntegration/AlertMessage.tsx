import { Color } from 'constants/designTokens';
import { Alert, Spin } from 'antd';
import { LoaderCircle, TriangleAlert } from 'lucide-react';
import { Callout } from 'components/ui/callout';

import { ModalStateEnum } from '../HeroSection/types';

import type { JSX } from 'react';

function AlertMessage({
	modalState,
}: {
	modalState: ModalStateEnum;
}): JSX.Element | null {
	switch (modalState) {
		case ModalStateEnum.WAITING:
			return (
				<Callout
					message={
						<div className="cloud-account-setup-form__alert-message">
							<Spin
								indicator={
									<LoaderCircle
										size={14}
										className="anticon anticon-loading anticon-spin ant-spin-dot"
									/>
								}
							/>
							Waiting for connection, retrying in{' '}
							<span className="retry-time">10</span> secs...
						</div>
					}
					type="info"
					showIcon={false}
				/>
			);
		case ModalStateEnum.ERROR:
			return (
				<Callout
					message={
						<div className="cloud-account-setup-form__alert-message">
							{`We couldn't establish a connection to your AWS account. Please try again`}
						</div>
					}
					type="error"
				/>
			);
		default:
			return null;
	}
}

export default AlertMessage;
