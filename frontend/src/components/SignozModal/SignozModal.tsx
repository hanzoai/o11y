import { Modal, ModalProps } from 'antd';

import './SignozModal.style.scss';

function O11yModal({
	children,
	width = 672,
	rootClassName = '',
	...rest
}: ModalProps): JSX.Element {
	return (
		<Modal
			centered
			width={width}
			cancelText="Close"
			rootClassName={`o11y-modal ${rootClassName}`}
			{...rest}
		>
			{children}
		</Modal>
	);
}

export default O11yModal;
