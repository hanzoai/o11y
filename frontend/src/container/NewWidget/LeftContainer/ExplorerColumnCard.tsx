import type { JSX } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Trash2 } from 'components/ui/icons';

type ExplorerColumnCardProps = {
	name: string;
	onRemove: (name: string) => void;
};

function ExplorerColumnCard({
	name,
	onRemove,
}: ExplorerColumnCardProps): JSX.Element {
	const { attributes, listeners, setNodeRef, transform, transition } =
		useSortable({ id: name });

	return (
		<div
			className="explorer-column-card"
			ref={setNodeRef}
			style={{ transform: CSS.Transform.toString(transform), transition }}
			{...attributes}
			{...listeners}
		>
			<div className="explorer-column-title">
				<GripVertical size={12} color="#5A5A5A" />
				{name}
			</div>
			<Trash2
				size={12}
				color="red"
				onClick={(): void => onRemove(name)}
				data-testid="trash-icon"
			/>
		</div>
	);
}

export default ExplorerColumnCard;
