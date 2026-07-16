import './index.css';
import * as React from 'react';
import { GripVertical } from 'lucide-react';
import {
	Group,
	Panel,
	Separator,
	useDefaultLayout,
	useGroupCallbackRef,
	useGroupRef,
	usePanelCallbackRef,
	usePanelRef,
} from 'react-resizable-panels';
import type {
	GroupImperativeHandle,
	Layout,
	LayoutStorage,
	OnGroupLayoutChange,
	PanelImperativeHandle,
	PanelSize,
} from 'react-resizable-panels';
import { cn } from '../lib/utils';

export type {
	GroupImperativeHandle,
	Layout,
	LayoutStorage,
	OnGroupLayoutChange,
	PanelImperativeHandle,
	PanelSize,
};
export {
	useDefaultLayout,
	useGroupCallbackRef,
	useGroupRef,
	usePanelCallbackRef,
	usePanelRef,
};

export type ResizablePanelGroupProps = React.ComponentProps<typeof Group> & {
	/**
	 * The testId associated with the panel group.
	 */
	testId?: string;
};

export const ResizablePanelGroup = React.forwardRef<
	HTMLDivElement,
	ResizablePanelGroupProps
>(({ className, testId, ...props }, ref) => (
	<Group
		data-slot="resizable-panel-group"
		className={cn('resizable-panel-group', className)}
		data-testid={testId}
		elementRef={ref}
		{...props}
	/>
));
ResizablePanelGroup.displayName = 'ResizablePanelGroup';

export type ResizablePanelProps = React.ComponentProps<typeof Panel> & {
	/**
	 * The testId associated with the panel.
	 */
	testId?: string;
};

export const ResizablePanel = React.forwardRef<
	HTMLDivElement,
	ResizablePanelProps
>(({ className, testId, ...props }, ref) => (
	<Panel
		data-slot="resizable-panel"
		className={className}
		data-testid={testId}
		elementRef={ref}
		{...props}
	/>
));
ResizablePanel.displayName = 'ResizablePanel';

export type ResizableHandleProps = React.ComponentProps<typeof Separator> & {
	/**
	 * The testId associated with the handle.
	 */
	testId?: string;
	/**
	 * Show a visible drag indicator.
	 */
	withHandle?: boolean;
};

export const ResizableHandle = React.forwardRef<
	HTMLDivElement,
	ResizableHandleProps
>(({ withHandle, className, testId, ...props }, ref) => (
	<Separator
		data-slot="resizable-handle"
		className={cn('resizable-handle', className)}
		data-testid={testId}
		elementRef={ref}
		{...props}
	>
		{withHandle && (
			<div
				data-slot="resizable-handle-icon-wrapper"
				className="resizable-handle-icon-wrapper"
			>
				<GripVertical
					data-slot="resizable-handle-icon"
					className="resizable-handle-icon"
				/>
			</div>
		)}
	</Separator>
));
ResizableHandle.displayName = 'ResizableHandle';
