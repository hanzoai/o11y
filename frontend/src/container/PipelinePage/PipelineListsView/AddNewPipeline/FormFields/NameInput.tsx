import { useTranslation } from 'react-i18next';
import { Input } from 'components/ui/input';
import { Form } from 'antd';
import { ProcessorFormField } from '../../AddNewProcessor/config';
import { formValidationRules } from '../../config';

import type { JSX } from 'react';

function NameInput({ fieldData }: NameInputProps): JSX.Element {
	const { t } = useTranslation('pipeline');

	return (
		<Form.Item
			required={false}
			label={fieldData.fieldName}
			key={fieldData.id}
			rules={formValidationRules}
			name={fieldData.name}
		>
			<Input placeholder={t(fieldData.placeholder)} />
		</Form.Item>
	);
}

interface NameInputProps {
	fieldData: ProcessorFormField;
}
export default NameInput;
