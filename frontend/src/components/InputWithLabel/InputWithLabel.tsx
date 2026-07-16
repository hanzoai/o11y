import { useState, type JSX } from 'react';
import { Input } from 'components/ui/input';
import { Button } from 'antd';
import { Typography } from 'components/ui/typography';
import cx from 'classnames';
import { X } from 'components/ui/icons';

import './InputWithLabel.styles.scss';

function InputWithLabel({
	label,
	initialValue = undefined,
	placeholder,
	type = 'text',
	onClose = undefined,
	labelAfter = false,
	onChange,
	className = undefined,
	closeIcon = undefined,
}: {
	label: string;
	initialValue?: string | number | null;
	placeholder: string;
	type?: string;
	onClose?: () => void;
	labelAfter?: boolean;
	onChange: (value: string) => void;
	className?: string;
	closeIcon?: React.ReactNode;
}): JSX.Element {
	const [inputValue, setInputValue] = useState<string>(
		initialValue ? initialValue.toString() : '',
	);

	const handleChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
		setInputValue(e.target.value);
		onChange?.(e.target.value);
	};

	return (
		<div
			className={cx('input-with-label', className, {
				labelAfter,
			})}
		>
			{!labelAfter && <Typography.Text className="label">{label}</Typography.Text>}
			<Input
				className="input"
				placeholder={placeholder}
				type={type}
				value={inputValue}
				onChange={handleChange}
				name={label.toLowerCase()}
				data-testid={`input-${label}`}
			/>
			{labelAfter && <Typography.Text className="label">{label}</Typography.Text>}
			{onClose && (
				<Button
					className="periscope-btn ghost close-btn"
					icon={closeIcon || <X size={16} />}
					onClick={onClose}
				/>
			)}
		</div>
	);
}

export default InputWithLabel;
