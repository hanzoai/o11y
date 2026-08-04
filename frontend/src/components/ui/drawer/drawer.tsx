import './index.css';
import { X } from 'lucide-react';
import type * as React from 'react';
import { Drawer as DrawerPrimitive } from 'vaul';

import { cn } from '@hanzo/ui/core';

function Drawer({
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Root>) {
	return <DrawerPrimitive.Root data-slot="drawer" {...props} />;
}

function DrawerTrigger({
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Trigger>) {
	return <DrawerPrimitive.Trigger data-slot="drawer-trigger" {...props} />;
}

function DrawerPortal({
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Portal>) {
	return <DrawerPrimitive.Portal data-slot="drawer-portal" {...props} />;
}

function DrawerClose({
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Close>) {
	return <DrawerPrimitive.Close data-slot="drawer-close" {...props} />;
}

function DrawerOverlay({
	className,
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Overlay>) {
	return (
		<DrawerPrimitive.Overlay
			data-slot="drawer-overlay"
			className={cn(className)}
			{...props}
		/>
	);
}
function DrawerContent({
	className,
	children,
	showOverlay = true,
	type,
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Content> & {
	showOverlay?: boolean;
	type?: 'panel' | 'drawer';
}) {
	return (
		<DrawerPortal data-slot="drawer-portal">
			{showOverlay && <DrawerOverlay />}
			<DrawerPrimitive.Content
				data-slot="drawer-content"
				data-type={type ?? 'panel'}
				className={cn(className)}
				{...props}
			>
				{children}
			</DrawerPrimitive.Content>
		</DrawerPortal>
	);
}

function DrawerHeader({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="drawer-header" className={cn(className)} {...props} />;
}

function DrawerFooter({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="drawer-footer" className={cn(className)} {...props} />;
}

function DrawerTitle({
	className,
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Title>) {
	return (
		<DrawerPrimitive.Title
			data-slot="drawer-title"
			className={cn(className)}
			{...props}
		/>
	);
}

function DrawerDescription({
	className,
	...props
}: React.ComponentProps<typeof DrawerPrimitive.Description>) {
	return (
		<DrawerPrimitive.Description
			data-slot="drawer-description"
			className={cn(className)}
			{...props}
		/>
	);
}

type DrawerWidth = 'narrow' | 'base' | 'wide' | 'extra-wide';

const DRAWER_WIDTH_PX: Record<DrawerWidth, string> = {
	narrow: '400px',
	base: '560px',
	wide: '720px',
	'extra-wide': '920px',
};

interface DrawerWrapperProps {
	/** Element that opens the drawer. Optional when using controlled mode (open/onOpenChange). */
	trigger?: React.ReactNode;
	header?: {
		title: string;
		description?: string;
	};
	/** Body of the drawer. Use `content` or pass JSX children — they are equivalent. */
	content?: React.ReactNode;
	children?: React.ReactNode;
	/** Convenience shorthand for `header.title` (antd-Drawer-compatible). */
	title?: string;
	/** Named width variant applied to the panel (antd-Drawer-compatible). */
	width?: DrawerWidth;
	footer?: React.ReactNode;
	direction?: 'top' | 'right' | 'bottom' | 'left';
	showCloseButton?: boolean;
	allowOutsideClick?: boolean;
	showOverlay?: boolean;
	className?: string;
	type?: 'panel' | 'drawer';
	/** Controlled open state. When provided with onOpenChange, enables programmatic control. */
	open?: boolean;
	/** Called when drawer open state changes (close button, outside click, ESC). Required for controlled mode. */
	onOpenChange?: (open: boolean) => void;
}

function CloseButton({ type }: { type?: 'panel' | 'drawer' }) {
	return (
		<DrawerClose asChild>
			<button type="button" data-slot="drawer-close-button" data-type={type}>
				<X size={16} />
				<span data-slot="drawer-close-label">Close</span>
			</button>
		</DrawerClose>
	);
}

function DrawerWrapper({
	trigger,
	header,
	content,
	children,
	title,
	width,
	footer,
	direction = 'right',
	showCloseButton = true,
	allowOutsideClick = true,
	showOverlay = true,
	className,
	type = 'drawer',
	open,
	onOpenChange,
}: DrawerWrapperProps) {
	const resolvedHeader = header ?? (title ? { title } : undefined);
	const body = content ?? children;
	const panelWidth = width
		? DRAWER_WIDTH_PX[width]
		: type === 'panel'
			? '720px'
			: 'auto';
	return (
		<Drawer
			direction={direction}
			modal={allowOutsideClick}
			open={open}
			onOpenChange={onOpenChange}
		>
			{trigger && <DrawerTrigger asChild>{trigger}</DrawerTrigger>}
			<DrawerContent className={className} showOverlay={showOverlay} type={type}>
				<div
					data-slot="drawer-panel"
					style={{
						width: panelWidth,
						height: type === 'panel' || width ? '100vh' : 'auto',
					}}
				>
					{resolvedHeader && (
						<div data-slot="drawer-titlebar">
							{type === 'panel' && showCloseButton && <CloseButton type={type} />}
							<div data-slot="drawer-titlebar-body">
								<DrawerTitle>{resolvedHeader.title}</DrawerTitle>
							</div>
							{type === 'drawer' && showCloseButton && <CloseButton type={type} />}
						</div>
					)}
					{resolvedHeader?.description && (
						<DrawerHeader>
							<DrawerDescription>{resolvedHeader.description}</DrawerDescription>
						</DrawerHeader>
					)}
					{body}
					{footer && <DrawerFooter>{footer}</DrawerFooter>}
				</div>
			</DrawerContent>
		</Drawer>
	);
}

export {
	Drawer,
	DrawerPortal,
	DrawerOverlay,
	DrawerTrigger,
	DrawerClose,
	DrawerContent,
	DrawerHeader,
	DrawerFooter,
	DrawerTitle,
	DrawerDescription,
	DrawerWrapper,
};
