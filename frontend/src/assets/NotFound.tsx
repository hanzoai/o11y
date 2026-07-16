import notFound404Url from '@/assets/Images/notFound404.png';

import type { JSX } from 'react';

function NotFound(): JSX.Element {
	return (
		<img
			style={{
				maxHeight: 480,
				maxWidth: 480,
			}}
			src={notFound404Url}
			alt="not-found"
		/>
	);
}

export default NotFound;
