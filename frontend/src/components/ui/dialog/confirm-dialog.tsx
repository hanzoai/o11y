import type React from 'react';
import { forwardRef, useState } from 'react';

import { Button, type ButtonProps } from '../button';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from './dialog';

export type ConfirmDialogProps = {
	testId?: string;
	id?: string;
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
	title: string;
	titleIcon?: React.ReactNode;
	children: React.ReactNode;
	className?: string;
	style?: React.CSSProperties;
	cancelText?: string;
	onCancel?: () => void;
	onConfirm: () => Promise<boolean | undefined | void> | boolean | undefined | void;
	cancelIcon?: React.ReactElement;
	confirmText: string;
	confirmColor?: ButtonProps['color'];
	confirmIcon?: React.ReactElement;
	disableOutsideClick?: boolean;
	width?: 'narrow' | 'base' | 'wide' | 'extra-wide';
	position?: 'center' | 'top';
	heightMode?: 'content' | 'fixed';
};

export const ConfirmDialog = forwardRef<HTMLDivElement, ConfirmDialogProps>((props, ref) => {
	const {
		testId,
		id,
		open,
		onOpenChange,
		title,
		titleIcon,
		children,
		className,
		style,
		cancelText = 'Cancel',
		onCancel,
		onConfirm,
		cancelIcon,
		confirmText,
		confirmColor = 'destructive',
		confirmIcon,
		disableOutsideClick = false,
		width = 'base',
	} = props;

	const [uncontrolledOpen, setUncontrolledOpen] = useState(true);
	const [loading, setLoading] = useState(false);
	const isControlled = open !== undefined;
	const isOpen = isControlled ? open : uncontrolledOpen;

	const setOpen = (next: boolean): void => {
		if (!isControlled) setUncontrolledOpen(next);
		onOpenChange?.(next);
	};

	const handleCancel = (): void => {
		onCancel?.();
		setOpen(false);
	};

	const handleConfirm = async (): Promise<void> => {
		setLoading(true);
		try {
			const result = await onConfirm();
			if (result === false) return;
			setOpen(false);
		} finally {
			setLoading(false);
		}
	};

	return (
		<Dialog open={isOpen} onOpenChange={setOpen}>
			<DialogContent
				ref={ref as never}
				id={id}
				data-testid={testId}
				className={className}
				style={style}
				width={width}
				showCloseButton={false}
				onPointerDownOutside={disableOutsideClick ? (e) => e.preventDefault() : undefined}
			>
				<DialogHeader>
					<DialogTitle icon={titleIcon}>{title}</DialogTitle>
				</DialogHeader>
				<DialogDescription>{children}</DialogDescription>
				<DialogFooter className="gap-3 p-4 pt-0">
					<Button variant="outlined" color="secondary" prefix={cancelIcon} onClick={handleCancel}>
						{cancelText}
					</Button>
					<Button
						variant="solid"
						color={confirmColor}
						loading={loading}
						prefix={confirmIcon}
						onClick={handleConfirm}
					>
						{confirmText}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
});
ConfirmDialog.displayName = 'ConfirmDialog';
