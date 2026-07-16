import { Badge } from 'components/ui/badge';

import type { JSX } from 'react';

function Tags({ tags }: TagsProps): JSX.Element {
	return (
		<span>
			{tags?.map((tag) => (
				<Badge color="sakura" key={tag}>
					{tag}
				</Badge>
			))}
		</span>
	);
}

interface TagsProps {
	tags: Array<string>;
}

export default Tags;
