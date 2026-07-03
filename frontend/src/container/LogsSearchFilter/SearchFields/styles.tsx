import type { HTMLAttributes } from 'react';
import { blue } from '@ant-design/colors';
import styled from 'styled-components';

export const QueryFieldContainer = styled.div<HTMLAttributes<HTMLDivElement>>`
	padding: 0.25rem 0.5rem;
	margin: 0.1rem 0.5rem 0;
	display: flex;
	flex-direction: row;
	align-items: center;
	border-radius: 0.25rem;
	gap: 1rem;
	width: 100%;
	&:hover {
		background: ${blue[6]};
	}
`;
