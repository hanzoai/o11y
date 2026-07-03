import type { HTMLAttributes, RefAttributes } from 'react';
import { Input } from 'antd';
import styled from 'styled-components';

const { Search } = Input;

export const Container = styled.div<
	HTMLAttributes<HTMLDivElement> & RefAttributes<HTMLDivElement>
>`
	display: flex;
	position: relative;
	width: 100%;
	margin-top: 1rem;
`;

export const SearchComponent = styled(Search)`
	.ant-btn-primary {
		svg {
			transform: scale(1.5);
		}
	}
`;
