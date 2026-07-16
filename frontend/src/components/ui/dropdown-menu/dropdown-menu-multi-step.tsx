import * as DropdownMenuPrimitive from '@radix-ui/react-dropdown-menu';
import { ChevronRight } from 'lucide-react';
import * as React from 'react';

import { cn } from '../lib/utils';
import {
	DropdownMenuBack,
	DropdownMenuPortal,
	DropdownMenuSeparator,
} from './dropdown-menu';

type OriginalContentProps = React.ComponentProps<
	typeof DropdownMenuPrimitive.Content
>;

type MultiStepContextValue = {
	currentStep: 'primary' | 'secondary';
	setCurrentStep: (step: 'primary' | 'secondary') => void;
};

const MultiStepContext = React.createContext<MultiStepContextValue | null>(
	null,
);

export type DropdownMenuMultiStepProps = {
	children?: React.ReactNode;
	open?: boolean;
	defaultOpen?: boolean;
	onOpenChange?: (open: boolean) => void;
	/** @default true */
	modal?: boolean;
};

/**
 * A multi-step dropdown menu that supports navigating between primary and secondary content.
 *
 * The forwarded ref is declared for API parity; the root renders only a Radix `Root`
 * (a context provider with no DOM node), so there is no element to attach it to.
 */
const DropdownMenuMultiStep = React.forwardRef<
	HTMLDivElement,
	DropdownMenuMultiStepProps
>(({ children, ...props }) => {
	const [currentStep, setCurrentStep] = React.useState<'primary' | 'secondary'>(
		'primary',
	);
	const [open, setOpen] = React.useState(false);

	React.useEffect(() => {
		if (!open) {
			setCurrentStep('primary');
		}
	}, [open]);

	return (
		<MultiStepContext.Provider value={{ currentStep, setCurrentStep }}>
			<DropdownMenuPrimitive.Root {...props} open={open} onOpenChange={setOpen}>
				{children}
			</DropdownMenuPrimitive.Root>
		</MultiStepContext.Provider>
	);
});
DropdownMenuMultiStep.displayName = 'DropdownMenuMultiStep';

export type DropdownMenuMultiStepContentProps = {
	className?: string;
	/** The content to display in the primary (initial) step. */
	primaryContent: React.ReactNode;
	/** The content to display in the secondary step. */
	secondaryContent: React.ReactNode;
	/** The label shown in the back button when in the secondary step. */
	secondaryLabel: string;
	/** @default 4 */
	sideOffset?: number;
	/** @default "bottom" */
	side?: OriginalContentProps['side'];
	/** @default "center" */
	align?: OriginalContentProps['align'];
};

/**
 * The content for a multi-step dropdown menu. Renders either primary or secondary content
 * based on the current step.
 */
const DropdownMenuMultiStepContent = React.forwardRef<
	HTMLDivElement,
	DropdownMenuMultiStepContentProps
>(
	(
		{
			primaryContent,
			secondaryContent,
			secondaryLabel,
			className,
			sideOffset = 4,
			...props
		},
		ref,
	) => {
		const context = React.useContext(MultiStepContext);
		if (!context) {
			throw new Error(
				'DropdownMenuMultiStepContent must be used within DropdownMenuMultiStep',
			);
		}
		const { currentStep, setCurrentStep } = context;

		const handleBack = () => {
			setCurrentStep('primary');
		};

		return (
			<DropdownMenuPortal>
				<DropdownMenuPrimitive.Content
					ref={ref}
					data-slot="dropdown-menu-multi-step-content"
					sideOffset={sideOffset}
					className={cn(className)}
					{...props}
				>
					{currentStep === 'primary' ? (
						primaryContent
					) : (
						<>
							<DropdownMenuBack label={secondaryLabel} onBack={handleBack} />
							<DropdownMenuSeparator />
							{secondaryContent}
						</>
					)}
				</DropdownMenuPrimitive.Content>
			</DropdownMenuPortal>
		);
	},
);
DropdownMenuMultiStepContent.displayName = 'DropdownMenuMultiStepContent';

export type DropdownMenuMultiStepTriggerProps = Omit<
	React.ComponentProps<typeof DropdownMenuPrimitive.Item>,
	'asChild' | 'onSelect'
> & {
	className?: string;
	/** Optional icon to display before the label. */
	leftIcon?: React.ReactNode;
};

/**
 * An item that triggers navigation to the secondary step in a multi-step dropdown.
 */
const DropdownMenuMultiStepTrigger = React.forwardRef<
	HTMLDivElement,
	DropdownMenuMultiStepTriggerProps
>(({ className, leftIcon, children, ...props }, ref) => {
	const context = React.useContext(MultiStepContext);
	if (!context) {
		throw new Error(
			'DropdownMenuMultiStepTrigger must be used within DropdownMenuMultiStep',
		);
	}
	const { setCurrentStep } = context;

	return (
		<DropdownMenuPrimitive.Item
			ref={ref}
			data-slot="dropdown-menu-multi-step-trigger"
			className={cn(className)}
			onSelect={(e) => {
				e.preventDefault();
				setCurrentStep('secondary');
			}}
			{...props}
		>
			{leftIcon && (
				<span data-slot="dropdown-menu-multi-step-trigger-icon">{leftIcon}</span>
			)}
			{children}
			<ChevronRight data-slot="dropdown-menu-multi-step-trigger-chevron" />
		</DropdownMenuPrimitive.Item>
	);
});
DropdownMenuMultiStepTrigger.displayName = 'DropdownMenuMultiStepTrigger';

export {
	DropdownMenuMultiStep,
	DropdownMenuMultiStepContent,
	DropdownMenuMultiStepTrigger,
};
