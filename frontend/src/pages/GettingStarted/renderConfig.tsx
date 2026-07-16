import { Typography } from 'components/ui/typography';
import Slack from 'container/SideNav/Slack';
import store from 'store';

import elixirPngUrl from '@/assets/Logos/elixir.png';
import goPngUrl from '@/assets/Logos/go.png';
import javaPngUrl from '@/assets/Logos/java.png';
import javascriptPngUrl from '@/assets/Logos/javascript.png';
import msNetFrameworkPngUrl from '@/assets/Logos/ms-net-framework.png';
import phpPngUrl from '@/assets/Logos/php.png';
import pythonPngUrl from '@/assets/Logos/python.png';
import railsPngUrl from '@/assets/Logos/rails.png';
import rustPngUrl from '@/assets/Logos/rust.png';

import { TGetStartedContentSection } from './types';
import {
	AlignLeft,
	BellRing,
	ChartBar,
	LayoutDashboard,
	Volume2,
	Unplug,
} from 'components/ui/icons';

export const GetStartedContent = (): TGetStartedContentSection[] => {
	const {
		app: { currentVersion },
	} = store.getState();
	return [
		{
			heading: 'Send data from your applications to Hanzo',
			items: [
				{
					title: 'Instrument your Java Application',
					icon: (
						<img src={`${javaPngUrl}?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/java/',
				},
				{
					title: 'Instrument your Python Application',
					icon: (
						<img src={`${pythonPngUrl}?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/python/',
				},
				{
					title: 'Instrument your JS Application',
					icon: (
						<img
							src={`${javascriptPngUrl}?currentVersion=${currentVersion}`}
							alt=""
						/>
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/javascript/',
				},
				{
					title: 'Instrument your Go Application',
					icon: (
						<img src={`/Logos/go.png?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/golang/',
				},
				{
					title: 'Instrument your .NET Application',
					icon: (
						<img
							src={`${msNetFrameworkPngUrl}?currentVersion=${currentVersion}`}
							alt=""
						/>
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/dotnet/',
				},
				{
					title: 'Instrument your PHP Application',
					icon: (
						<img src={`/Logos/php.png?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/php/',
				},
				{
					title: 'Instrument your Rails Application',
					icon: (
						<img src={`${railsPngUrl}?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/ruby-on-rails/',
				},
				{
					title: 'Instrument your Rust Application',
					icon: (
						<img src={`${rustPngUrl}?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/rust/',
				},
				{
					title: 'Instrument your Elixir Application',
					icon: (
						<img src={`${elixirPngUrl}?currentVersion=${currentVersion}`} alt="" />
					),
					url: 'https://o11y.hanzo.ai/docs/instrumentation/elixir/',
				},
			],
		},
		{
			heading: 'Send Metrics from your Infrastructure & create Dashboards',
			items: [
				{
					title: 'Send metrics to Hanzo',
					icon: <ChartBar style={{ fontSize: '3.5rem' }} />,
					url: 'https://o11y.hanzo.ai/docs/userguide/send-metrics/',
				},
				{
					title: 'Create and Manage Dashboards',
					icon: <LayoutDashboard style={{ fontSize: '3.5rem' }} />,
					url: 'https://o11y.hanzo.ai/docs/userguide/manage-dashboards-and-panels/',
				},
			],
		},
		{
			heading: 'Send your logs to Hanzo',
			items: [
				{
					title: 'Send your logs to Hanzo',
					icon: <AlignLeft style={{ fontSize: '3.5rem' }} />,
					url: 'https://o11y.hanzo.ai/docs/userguide/logs/',
				},
				{
					title: 'Existing log collectors to Hanzo',
					icon: <Unplug style={{ fontSize: '3.5rem' }} />,
					url: 'https://o11y.hanzo.ai/docs/userguide/fluentbit_to_o11y/',
				},
			],
		},
		{
			heading: 'Create alerts on Metrics',
			items: [
				{
					title: 'Create alert rules on metrics',
					icon: <BellRing style={{ fontSize: '3.5rem' }} />,
					url: 'https://o11y.hanzo.ai/docs/userguide/alerts-management/',
				},
				{
					title: 'Configure alert notification channels',
					icon: <Volume2 style={{ fontSize: '3.5rem' }} />,
					url: 'https://o11y.hanzo.ai/docs/userguide/alerts-management/#setting-up-a-notification-channel',
				},
			],
		},
		{
			heading: 'Need help?',
			description: (
				<>
					{'Join our slack community and ask any question you may have on '}
					<Typography.Link
						href="https://o11y-community.slack.com/archives/C01HWUTP4HH"
						target="_blank"
					>
						#support
					</Typography.Link>
					{' or '}
					<Typography.Link
						href="https://o11y-community.slack.com/archives/C01HWQ1R0BC"
						target="_blank"
					>
						#dummy_channel
					</Typography.Link>
				</>
			),

			items: [
				{
					title: 'Join Hanzo slack community ',
					icon: (
						<div style={{ padding: '0.7rem' }}>
							<Slack width={30} height={30} />
						</div>
					),
					url: 'https://o11y.hanzo.ai/slack',
				},
			],
		},
	];
};
