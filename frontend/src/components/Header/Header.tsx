import './Header.styles.scss';

import type { JSX } from 'react';

export default function Header({
	leftComponent,
	rightComponent,
}: {
	leftComponent: React.ReactNode;
	rightComponent: React.ReactNode | null;
}): JSX.Element {
	return (
		<div className="header-container">
			<div className="header-left">{leftComponent}</div>

			{rightComponent && <div className="header-right">{rightComponent}</div>}
		</div>
	);
}
