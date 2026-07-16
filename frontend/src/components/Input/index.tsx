import {
	ChangeEventHandler,
	FocusEventHandler,
	KeyboardEventHandler,
	LegacyRef,
	ReactNode,
	Ref,
	type JSX,
} from 'react';
import { Form, Input, InputProps, InputRef } from 'antd';

function InputComponent({
	value,
	type = 'text',
	onChangeHandler = undefined,
	placeholder = undefined,
	ref = undefined,
	size = 'small',
	onBlurHandler = undefined,
	onPressEnterHandler = undefined,
	label = undefined,
	labelOnTop = undefined,
	addonBefore = undefined,
	...props
}: InputComponentProps): JSX.Element {
	return (
		<Form.Item labelCol={{ span: labelOnTop ? 24 : 4 }} label={label}>
			<Input
				placeholder={placeholder}
				type={type}
				onChange={onChangeHandler}
				value={value}
				ref={ref as Ref<InputRef>}
				size={size}
				addonBefore={addonBefore}
				onBlur={onBlurHandler}
				onPressEnter={onPressEnterHandler}
				{...props}
			/>
		</Form.Item>
	);
}

interface InputComponentProps extends InputProps {
	value: InputProps['value'];
	type?: InputProps['type'];
	onChangeHandler?: ChangeEventHandler<HTMLInputElement>;
	placeholder?: InputProps['placeholder'];
	ref?: LegacyRef<InputRef>;
	size?: InputProps['size'];
	onBlurHandler?: FocusEventHandler<HTMLInputElement>;
	onPressEnterHandler?: KeyboardEventHandler<HTMLInputElement>;
	label?: string;
	labelOnTop?: boolean;
	addonBefore?: ReactNode;
}

export default InputComponent;
