import { ReactNode, useState, type JSX } from 'react';
import {
	ChevronDown,
	ChevronRight,
	EyeClosed,
	EyeOpen,
	Trash2,
} from 'components/ui/icons';
import { Button, Row } from 'antd';

import { QueryWrapper } from '../styles';

import './QueryHeader.styles.scss';

interface IQueryHeaderProps {
	disabled: boolean;
	onDisable: VoidFunction;
	name: string;
	deletable: boolean;
	onDelete: VoidFunction;
	children: ReactNode;
}

function QueryHeader({
	disabled,
	onDisable,
	name,
	onDelete,
	deletable,
	children,
}: IQueryHeaderProps): JSX.Element {
	const [collapse, setCollapse] = useState(false);
	return (
		<QueryWrapper className="query-header-container">
			<Row style={{ justifyContent: 'space-between', marginBottom: '0.4rem' }}>
				<Row>
					<Button
						type="default"
						icon={disabled ? <EyeClosed size={16} /> : <EyeOpen size={16} />}
						onClick={onDisable}
						className="action-btn"
					>
						{name}
					</Button>
					<Button
						type="default"
						icon={collapse ? <ChevronRight size={16} /> : <ChevronDown size={16} />}
						onClick={(): void => setCollapse(!collapse)}
						className="action-btn"
					/>
				</Row>

				{deletable && (
					<Button
						type="default"
						danger
						icon={<Trash2 size={16} />}
						onClick={onDelete}
						className="action-btn"
					/>
				)}
			</Row>
			{!collapse && children}
		</QueryWrapper>
	);
}

export default QueryHeader;
