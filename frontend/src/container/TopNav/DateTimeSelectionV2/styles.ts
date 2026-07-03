import type { HTMLAttributes } from 'react';
import { Form as FormComponent } from 'antd';
import { Typography as TypographyComponent } from 'components/ui/typography';
import styled from 'styled-components';

export const Form = styled(FormComponent)`
	&&& {
		justify-content: flex-end;
	}
`;

export const Typography = styled(TypographyComponent)`
	&&& {
		text-align: right;
	}
`;

export const FormItem = styled(Form.Item)`
	&&& {
		margin: 0;
	}
`;

interface Props extends HTMLAttributes<HTMLDivElement> {
	refreshButtonHidden: boolean;
}

export const RefreshTextContainer = styled.div<Props>`
	padding-right: 8px;
	visibility: ${({ refreshButtonHidden }): string =>
		refreshButtonHidden ? 'hidden' : 'visible'};
`;

export const FormContainer = styled.div`
	display: flex;
`;
