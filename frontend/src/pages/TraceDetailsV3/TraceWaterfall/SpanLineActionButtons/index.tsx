import { Button } from 'components/ui/button';
import {
	TooltipRoot,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from 'components/ui/tooltip';
import { useCopySpanLink } from 'hooks/trace/useCopySpanLink';
import { Link } from 'components/ui/icons';
import { Span } from 'types/api/trace/getTraceV2';

import styles from './SpanLineActionButtons.module.scss';

import type { JSX } from 'react';

export interface SpanLineActionButtonsProps {
	span: Span;
}
export default function SpanLineActionButtons({
	span,
}: SpanLineActionButtonsProps): JSX.Element {
	const { onSpanCopy } = useCopySpanLink(span);

	return (
		<div className={styles.root}>
			<TooltipProvider>
				<TooltipRoot>
					<TooltipTrigger asChild>
						<Button
							variant="ghost"
							size="icon"
							color="secondary"
							onClick={onSpanCopy}
							className={styles.copyBtn}
						>
							<Link size={14} />
						</Button>
					</TooltipTrigger>
					<TooltipContent className={styles.tooltip}>Copy Span Link</TooltipContent>
				</TooltipRoot>
			</TooltipProvider>
		</div>
	);
}
