import { ReactElement, type JSX } from 'react';
import { Modal, ModalProps as Props } from 'antd';

function CustomModal({
	title,
	children,
	isModalVisible,
	footer = undefined,
	closable = true,
}: ModalProps): JSX.Element {
	return (
		<Modal
			title={title}
			open={isModalVisible}
			footer={footer}
			closable={closable}
		>
			{children}
		</Modal>
	);
}

interface ModalProps {
	isModalVisible: boolean;
	closable?: boolean;
	footer?: Props['footer'];
	title: string;
	children: ReactElement;
}

export default CustomModal;
