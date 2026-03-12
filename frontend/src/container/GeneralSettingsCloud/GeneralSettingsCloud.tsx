import { Card, Typography } from 'antd';
import { Info } from 'lucide-react';

import './GeneralSettingsCloud.styles.scss';

export default function GeneralSettingsCloud(): JSX.Element {
	return (
		<Card className="general-settings-container">
			<Info size={16} />
			<Typography.Text>
				Please <a href="mailto:cloud-support@o11y.hanzo.ai"> email us </a> or connect
				with us via chat support to change the retention period.
			</Typography.Text>
		</Card>
	);
}
