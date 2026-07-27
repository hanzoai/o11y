import type { HTMLAttributes } from 'react';
import styled from 'styled-components';

interface Props extends HTMLAttributes<HTMLDivElement> {
	isDashboardPage: boolean;
}

interface ValueContainerProps extends HTMLAttributes<HTMLDivElement> {
	showClickable?: boolean;
}

export const ValueContainer = styled.div<ValueContainerProps>`
	height: 100%;
	display: flex;
	justify-content: center;
	align-items: center;
	flex-direction: column;
	user-select: none;
	cursor: ${({ showClickable = false }): string =>
		showClickable ? 'pointer' : 'default'};
`;

export const TitleContainer = styled.div<Props>`
	text-align: center;
	padding-top: ${({ isDashboardPage }): string =>
		!isDashboardPage ? '1rem' : '0rem'};
`;
