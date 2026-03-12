import React from 'react';
import { ArrowUpRight } from 'lucide-react';

import './AuthFooter.styles.scss';

interface FooterItem {
	icon?: string;
	text: string;
	url?: string;
	statusIndicator?: boolean;
}

const footerItems: FooterItem[] = [
	{
		text: 'All systems operational',
		url: 'https://status.o11y.hanzo.ai/',
		statusIndicator: true,
	},
	{
		text: 'Privacy',
		url: 'https://www.o11y.hanzo.ai/privacy',
	},
	{
		text: 'Security',
		url: 'https://www.o11y.hanzo.ai/security',
	},
];

function AuthFooter(): JSX.Element {
	return (
		<footer className="auth-footer">
			<div className="auth-footer-content">
				{footerItems.map((item, index) => (
					<React.Fragment key={item.text}>
						<div className="auth-footer-item">
							{item.statusIndicator && (
								<div className="auth-footer-status-indicator" />
							)}
							{item.icon && (
								<img
									loading="lazy"
									src={item.icon}
									alt=""
									className="auth-footer-icon"
								/>
							)}
							{item.url ? (
								<a
									href={item.url}
									className={`auth-footer-link ${
										item.statusIndicator ? 'auth-footer-link-status' : ''
									}`}
									target="_blank"
									rel="noopener noreferrer"
								>
									<span className="auth-footer-text">{item.text}</span>
									{!item.statusIndicator && (
										<ArrowUpRight size={12} className="auth-footer-link-icon" />
									)}
								</a>
							) : (
								<span className="auth-footer-text">{item.text}</span>
							)}
						</div>
						{index < footerItems.length - 1 && (
							<div className="auth-footer-separator" />
						)}
					</React.Fragment>
				))}
			</div>
		</footer>
	);
}

export default AuthFooter;
