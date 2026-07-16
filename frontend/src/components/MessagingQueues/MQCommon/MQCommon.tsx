import { Color } from 'constants/designTokens';
import { Tooltip, type SelectProps } from 'antd';
import { Info } from 'components/ui/icons';

import './MQCommon.styles.scss';

import type { JSX } from 'react';

export function ComingSoon(): JSX.Element {
	return (
		<Tooltip
			title={
				<div>
					Join our Slack community for more details:{' '}
					<a
						href="https://o11y.hanzo.ai/slack"
						rel="noopener noreferrer"
						target="_blank"
						onClick={(e): void => e.stopPropagation()}
					>
						Hanzo Community
					</a>
				</div>
			}
			placement="top"
			overlayClassName="tooltip-overlay"
		>
			<div className="coming-soon">
				<div className="coming-soon__text">Coming Soon</div>
				<div className="coming-soon__icon">
					<Info size={10} color={Color.BG_SIENNA_400} />
				</div>
			</div>
		</Tooltip>
	);
}

// antd does not re-export rc-select's DisplayValueType — derive it from the prop.
type OmittedValues = Parameters<
	Extract<SelectProps['maxTagPlaceholder'], (...args: never[]) => unknown>
>[0];

export function SelectMaxTagPlaceholder(
	omittedValues: OmittedValues,
): JSX.Element {
	return (
		<Tooltip title={omittedValues.map(({ value }) => value).join(', ')}>
			<span>+ {omittedValues.length} </span>
		</Tooltip>
	);
}

export function SelectLabelWithComingSoon({
	label,
}: {
	label: string;
}): JSX.Element {
	return (
		<div className="select-label-with-coming-soon">
			{label} <ComingSoon />
		</div>
	);
}
