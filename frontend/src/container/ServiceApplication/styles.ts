import type { HTMLAttributes } from 'react';
import { Typography } from 'components/ui/typography';
import styled from 'styled-components';

export const Container = styled.div<HTMLAttributes<HTMLDivElement>>`
	margin-top: 2rem;
`;

export const Name = styled(Typography)`
	&&& {
		font-weight: 600;
		color: #4e74f8;
		cursor: pointer;
	}
`;
