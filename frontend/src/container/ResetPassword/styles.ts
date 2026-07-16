import { Form } from 'antd';
import styled from 'styled-components';

import type { FormValues } from './types';

export const FormContainer = styled(Form<FormValues>)`
	& .ant-form-item {
		margin-bottom: 0px;
	}
`;
