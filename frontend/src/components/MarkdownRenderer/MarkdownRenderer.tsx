/* eslint-disable @typescript-eslint/explicit-function-return-type */

import ReactMarkdown, { type ExtraProps } from 'react-markdown';
import logEvent from 'api/common/logEvent';
import { isEmpty } from 'lodash-es';
import rehypeRaw from 'rehype-raw';

import CodeCopyBtn from './CodeCopyBtn/CodeCopyBtn';
import SyntaxHighlighter, { a11yDark } from './syntaxHighlighter';

import type { ComponentProps, JSX } from 'react';

interface LinkProps {
	href: string;
	children: React.ReactElement;
}

function Pre({
	children,
	elementDetails,
	trackCopyAction,
}: {
	children: React.ReactNode;
	trackCopyAction: boolean;
	elementDetails: Record<string, unknown>;
}): JSX.Element {
	const { trackingTitle = '', ...rest } = elementDetails;

	const handleClick = (additionalInfo?: Record<string, unknown>): void => {
		const trackingData = { ...rest, copiedContent: additionalInfo };

		if (trackCopyAction && !isEmpty(trackingTitle)) {
			logEvent(trackingTitle as string, trackingData);
		}
	};

	return (
		<pre className="code-snippet-container">
			<CodeCopyBtn onCopyClick={handleClick}>{children}</CodeCopyBtn>
			{children}
		</pre>
	);
}

function Code({
	className = 'blog-code',
	children,
	...props
}: ComponentProps<'code'> & ExtraProps): JSX.Element {
	const match = /language-(\w+)/.exec(className || '');
	return match ? (
		<SyntaxHighlighter
			// @ts-expect-error
			style={a11yDark}
			language={match[1]}
			PreTag="div"
			{...props}
		>
			{String(children).replace(/\n$/, '')}
		</SyntaxHighlighter>
	) : (
		<code className={className} {...props}>
			{children}
		</code>
	);
}

function Link({ href, children }: LinkProps): JSX.Element {
	return (
		<a href={href} target="_blank" rel="noopener noreferrer">
			{children}
		</a>
	);
}

const interpolateMarkdown = (
	markdownContent: any,
	variables: { [s: string]: unknown } | ArrayLike<unknown>,
) => {
	let interpolatedContent = markdownContent;

	const variableEntries = Object.entries(variables);

	// Loop through variables and replace placeholders with values
	for (const [key, value] of variableEntries) {
		const placeholder = `{{${key}}}`;
		const regex = new RegExp(placeholder, 'g');
		interpolatedContent = interpolatedContent.replace(regex, value);
	}

	return interpolatedContent;
};

function CustomTag({ color }: { color: string }): JSX.Element {
	return <h1 style={{ color }}>This is custom element</h1>;
}

function MarkdownRenderer({
	markdownContent,
	variables,
	trackCopyAction = false,
	elementDetails = {},
	className,
}: {
	markdownContent: any;
	variables: any;
	trackCopyAction?: boolean;
	elementDetails?: Record<string, unknown>;
	className?: string;
}): JSX.Element {
	const interpolatedMarkdown = interpolateMarkdown(markdownContent, variables);

	return (
		<ReactMarkdown
			className={className}
			rehypePlugins={[rehypeRaw]}
			components={{
				// @ts-expect-error
				a: Link,
				pre: ({ children }) =>
					Pre({
						children,
						elementDetails: elementDetails ?? {},
						trackCopyAction: !!trackCopyAction,
					}),
				code: Code,
				customtag: CustomTag,
			}}
		>
			{interpolatedMarkdown}
		</ReactMarkdown>
	);
}

export { Code, Link, MarkdownRenderer, Pre };
