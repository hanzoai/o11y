import { ToggleGroupSimple } from 'components/ui/toggle-group';

import './O11yRadioGroup.styles.scss';

import type { JSX } from 'react';

interface Option {
	value: string;
	label: string | React.ReactNode;
	icon?: React.ReactNode;
}

interface O11yRadioGroupProps {
	value: string;
	options: Option[];
	onChange: (value: string) => void;
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
		<ToggleGroupSimple
			type="single"
			value={value}
			className={`o11y-radio-group ${className}`}
			onChange={onChange}
			disabled={disabled}
			items={options.map((option) => ({
				value: option.value,
				label: (
					<div className="view-title-container">
						{option.icon && <div className="icon-container">{option.icon}</div>}
						{option.label}
					</div>
				),
			}))}
		/>
	);
}

export default O11yRadioGroup;
