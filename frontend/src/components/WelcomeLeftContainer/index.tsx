import { ReactChild } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Space, Typography } from 'antd';
import BrandMark from 'components/BrandMark';
import { useTenant } from 'providers/Tenant';

import { Container, LeftContainer } from './styles';

function WelcomeLeftContainer({
	version,
	children,
}: WelcomeLeftContainerProps): JSX.Element {
	const { t } = useTranslation();
	const tenant = useTenant();

	return (
		<Container>
			<LeftContainer direction="vertical">
				<Space align="center">
					<BrandMark size={46} />
				</Space>
				<Typography>{t('monitor_signup')}</Typography>
				<Card
					style={{ width: 'max-content' }}
					bodyStyle={{ padding: '1px 8px', width: '100%' }}
				>
					{tenant.name} {version}
				</Card>
			</LeftContainer>
			{children}
		</Container>
	);
}

interface WelcomeLeftContainerProps {
	version: string;
	children: ReactChild;
}

export default WelcomeLeftContainer;
