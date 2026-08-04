import './index.css';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { XIcon } from 'lucide-react';
import * as React from 'react';
import { cn } from '@hanzo/ui/core';

import { Checkbox } from '../checkbox/checkbox';

function Dialog({
	...props
}: React.ComponentProps<typeof DialogPrimitive.Root>) {
	return <DialogPrimitive.Root data-slot="dialog" {...props} />;
}

function DialogTrigger({
	className,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
	return (
		<DialogPrimitive.Trigger
			data-slot="dialog-trigger"
			className={cn(className)}
			{...props}
		/>
	);
}

function DialogPortal({
	...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
	return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />;
}

function DialogClose({
	className,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
	return (
		<DialogPrimitive.Close
			data-slot="dialog-close"
			className={cn(className)}
			{...props}
		/>
	);
}

function DialogOverlay({
	className,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
	return (
		<DialogPrimitive.Overlay
			data-slot="dialog-overlay"
			className={cn(className)}
			{...props}
		/>
	);
}

function DialogContent({
	className,
	children,
	showCloseButton = true,
	width = 'base',
	...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
	showCloseButton?: boolean;
	width?: 'narrow' | 'base' | 'wide' | 'extra-wide';
}) {
	return (
		<DialogPortal data-slot="dialog-portal">
			<DialogOverlay />
			<DialogPrimitive.Content
				data-slot="dialog-content"
				data-width={width}
				className={cn(className)}
				{...props}
			>
				{children}
				{showCloseButton && (
					<DialogPrimitive.Close data-slot="dialog-close-corner">
						<XIcon size={14} />
						<span data-slot="dialog-close-label">Close</span>
					</DialogPrimitive.Close>
				)}
			</DialogPrimitive.Content>
		</DialogPortal>
	);
}

function DialogHeader({ className, ...props }: React.ComponentProps<'div'>) {
	return (
		<div
			data-slot="dialog-header"
			className={cn(className)}
			{...props}
		/>
	);
}

function DialogFooter({ className, ...props }: React.ComponentProps<'div'>) {
	return (
		<div
			data-slot="dialog-footer"
			className={cn(className)}
			{...props}
		/>
	);
}

function DialogTitle({
	className,
	icon,
	children,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Title> & {
	icon?: React.ReactNode;
}) {
	return (
		<DialogPrimitive.Title
			data-slot="dialog-title"
			className={cn(className)}
			{...props}
		>
			{icon}
			{children}
		</DialogPrimitive.Title>
	);
}

function DialogDescription({
	className,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
	return (
		<DialogPrimitive.Description
			data-slot="dialog-description"
			className={cn(className)}
			{...props}
		/>
	);
}

interface DialogWrapperProps {
	title?: string;
	children: React.ReactNode;
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
	trigger?: React.ReactNode;
	className?: string;
	showCloseButton?: boolean;
	disableOutsideClick?: boolean;
	width?: 'narrow' | 'base' | 'wide' | 'extra-wide';
	titleIcon?: React.ReactNode;
	footer?: React.ReactNode;
	style?: React.CSSProperties;
}

function DialogWrapper({
	title,
	children,
	open,
	onOpenChange,
	trigger,
	className,
	showCloseButton = true,
	disableOutsideClick = false,
	width = 'base',
	titleIcon,
	footer,
	style,
}: DialogWrapperProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			{trigger && <DialogTrigger asChild>{trigger}</DialogTrigger>}
			<DialogContent
				className={className}
				style={style}
				showCloseButton={showCloseButton}
				onPointerDownOutside={
					disableOutsideClick ? (e) => e.preventDefault() : undefined
				}
				width={width}
			>
				{title && (
					<DialogHeader>
						{title && <DialogTitle icon={titleIcon}>{title}</DialogTitle>}
					</DialogHeader>
				)}
				{children && <DialogDescription>{children}</DialogDescription>}
				{footer && <DialogFooter>{footer}</DialogFooter>}
			</DialogContent>
		</Dialog>
	);
}

type CheckboxColor =
	| 'robin'
	| 'forest'
	| 'amber'
	| 'sienna'
	| 'cherry'
	| 'sakura'
	| 'aqua';

interface AlertDialogContentProps {
	title?: string;
	titleIcon?: React.ReactNode;
	children: React.ReactNode;
	checkboxLabel?: string;
	checkboxChecked?: boolean;
	onCheckboxChange?: (checked: boolean) => void;
	checkboxColor?: CheckboxColor;
	footer?: React.ReactNode;
}

function AlertDialogContent({
	title,
	titleIcon,
	children,
	checkboxLabel,
	checkboxChecked,
	onCheckboxChange,
	checkboxColor = 'cherry',
	footer,
}: AlertDialogContentProps) {
	const checkboxId = React.useId();

	return (
		<div data-slot="alert-dialog-body">
			<div data-slot="alert-dialog-block">
				{title && (
					<DialogHeader>
						<DialogTitle icon={titleIcon}>{title}</DialogTitle>
					</DialogHeader>
				)}
				{children && <DialogDescription>{children}</DialogDescription>}
				{checkboxLabel && (
					<Checkbox
						id={checkboxId}
						color={checkboxColor}
						value={checkboxChecked}
						onChange={(checked): void => onCheckboxChange?.(checked === true)}
					>
						{checkboxLabel}
					</Checkbox>
				)}
			</div>
			{footer && <DialogFooter>{footer}</DialogFooter>}
		</div>
	);
}

interface AlertDialogWrapperProps extends Omit<
	DialogWrapperProps,
	'showCloseButton' | 'disableOutsideClick'
> {
	checkboxLabel?: string;
	checkboxChecked?: boolean;
	onCheckboxChange?: (checked: boolean) => void;
	checkboxColor?: CheckboxColor;
	footer?: React.ReactNode;
}

export function AlertDialogWrapper({
	children,
	checkboxLabel,
	checkboxChecked,
	onCheckboxChange,
	checkboxColor = 'cherry',
	footer,
	title,
	titleIcon,
	...props
}: AlertDialogWrapperProps) {
	return (
		<DialogWrapper
			className="alert-dialog"
			showCloseButton={false}
			disableOutsideClick={true}
			{...props}
		>
			<AlertDialogContent
				title={title}
				titleIcon={titleIcon}
				checkboxLabel={checkboxLabel}
				checkboxChecked={checkboxChecked}
				onCheckboxChange={onCheckboxChange}
				checkboxColor={checkboxColor}
				footer={footer}
			>
				{children}
			</AlertDialogContent>
		</DialogWrapper>
	);
}

export {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogOverlay,
	DialogPortal,
	DialogTitle,
	DialogTrigger,
	DialogWrapper,
};
