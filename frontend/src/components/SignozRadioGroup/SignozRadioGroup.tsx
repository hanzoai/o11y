import { Radio, RadioChangeEvent } from 'antd';

import './O11yRadioGroup.styles.scss';

interface Option {
	value: string;
	label: string | React.ReactNode;
	icon?: React.ReactNode;
}

interface O11yRadioGroupProps {
	value: string;
	options: Option[];
	onChange: (e: RadioChangeEvent) => void;
	className?: string;
	disabled?: boolean;
}

function O11yRadioGroup({
	value,
	options,
	onChange,
	className = '',
	disabled = false,
}: O11yRadioGroupProps): JSX.Element {
	return (
		<Radio.Group
			value={value}
			buttonStyle="solid"
			className={`o11y-radio-group ${className}`}
			onChange={onChange}
			disabled={disabled}
		>
			{options.map((option) => (
				<Radio.Button
					key={option.value}
					value={option.value}
					className={value === option.value ? 'selected_view tab' : 'tab'}
				>
					<div className="view-title-container">
						{option.icon && <div className="icon-container">{option.icon}</div>}
						{option.label}
					</div>
				</Radio.Button>
			))}
		</Radio.Group>
	);
}

O11yRadioGroup.defaultProps = {
	className: '',
	disabled: false,
};

export default O11yRadioGroup;
