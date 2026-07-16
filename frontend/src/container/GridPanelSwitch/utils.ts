import React, { PropsWithChildren, ReactNode } from 'react';

export const generateGridTitle = (title: ReactNode): string => {
	if (React.isValidElement<PropsWithChildren>(title)) {
		const { children } = title.props;
		return (Array.isArray(children) ? children : [children])
			.map((child: ReactNode) => (typeof child === 'string' ? child : ''))
			.join(' ');
	}
	return title?.toString() || '';
};
