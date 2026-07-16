import { GripVertical } from 'components/ui/icons';
import { Switch } from 'components/ui/switch';

import { holdIconStyle } from '../config';
import { LastActionColumn } from '../styles';

import type { JSX } from 'react';

function DragAction({ isEnabled, onChange }: DragActionProps): JSX.Element {
	return (
		<LastActionColumn>
			<Switch defaultValue={isEnabled} onChange={onChange} />
			<GripVertical size="lg" style={holdIconStyle} />
		</LastActionColumn>
	);
}

interface DragActionProps {
	isEnabled: boolean;
	onChange: (checked: boolean) => void;
}

export default DragAction;
