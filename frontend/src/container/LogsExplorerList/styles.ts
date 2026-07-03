import type { HTMLAttributes } from 'react';
import styled from 'styled-components';

export const InfinityWrapperStyled = styled.div<
	HTMLAttributes<HTMLDivElement> & { 'data-testid'?: string }
>`
	flex: 1;
	display: flex;
	height: 100%;
	min-height: 0;
`;
