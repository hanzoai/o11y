import React, { useState, type JSX } from 'react';
import { Check, Copy } from 'components/ui/icons';
import cx from 'classnames';

import './CodeCopyBtn.scss';

function CodeCopyBtn({
	children,
	onCopyClick = (): void => {},
}: {
	children: React.ReactNode;
	onCopyClick?: (additionalInfo?: Record<string, unknown>) => void;
}): JSX.Element {
	const [isSnippetCopied, setIsSnippetCopied] = useState(false);

	const handleClick = (): void => {
		let copiedText = '';
		if (children && Array.isArray(children)) {
			setIsSnippetCopied(true);
			// oxlint-disable-next-line o11y/no-navigator-clipboard
			navigator.clipboard.writeText(children[0].props.children[0]).finally(() => {
				copiedText = (children[0].props.children[0] as string).slice(0, 200); // slicing is done due to the limitation in accepted char length in attributes
				setTimeout(() => {
					setIsSnippetCopied(false);
				}, 1000);
			});
			copiedText = (children[0].props.children[0] as string).slice(0, 200);
		}

		onCopyClick?.({ copiedText });
	};

	return (
		<div className={cx('code-copy-btn', isSnippetCopied ? 'copied' : '')}>
			<button type="button" onClick={handleClick}>
				{!isSnippetCopied ? <Copy size={16} /> : <Check size={16} />}
			</button>
		</div>
	);
}

export default CodeCopyBtn;
