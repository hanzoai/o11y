import { Row } from 'antd';
import { HTMLAttributes } from 'react';
import styled from 'styled-components';

export const VariableItemRow = styled(Row)`
	gap: 1rem;
	margin-bottom: 1rem;
`;

export const LabelContainer = styled.div<HTMLAttributes<HTMLDivElement>>`
	width: 200px;
`;
