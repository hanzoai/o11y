import type { HTMLAttributes } from 'react';
import styled from 'styled-components';

export const TitleWrapper = styled.span<HTMLAttributes<HTMLSpanElement>>`
	user-select: text !important;
	cursor: text;

	.hover-reveal {
		visibility: hidden;
	}

	&:hover .hover-reveal {
		visibility: visible;
	}
`;
