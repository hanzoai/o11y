import './index.css';
import * as React from 'react';
import { ChevronLeft, ChevronRight, Minus } from 'lucide-react';
import { cn } from '../lib/utils';

/* -------------------------------------------------------------------------- */
/* Page-number sequence helper                                                */
/* -------------------------------------------------------------------------- */

const range = (start: number, end: number): number[] => {
	const length = end - start + 1;
	return Array.from({ length }, (_, i) => start + i);
};

/**
 * Calculates the sequence of page numbers and ellipses to display.
 */
export const renderPageNumbers = (
	totalPages: number,
	current: number,
	siblingCount = 1
): (number | 'ellipsis')[] => {
	if (totalPages <= 2 * siblingCount + 5) return range(1, totalPages);
	const leftSiblingIndex = Math.max(current - siblingCount, 1);
	const rightSiblingIndex = Math.min(current + siblingCount, totalPages);
	const shouldShowLeftDots = leftSiblingIndex > 2;
	const shouldShowRightDots = rightSiblingIndex < totalPages - 1;
	const firstPageIndex = 1;
	const lastPageIndex = totalPages;
	if (!shouldShowLeftDots && shouldShowRightDots) {
		return [...range(1, 3), 'ellipsis', lastPageIndex];
	}
	if (shouldShowLeftDots && !shouldShowRightDots) {
		return [firstPageIndex, 'ellipsis', ...range(totalPages - 2, totalPages)];
	}
	if (shouldShowLeftDots && shouldShowRightDots) {
		return [
			firstPageIndex,
			'ellipsis',
			...range(leftSiblingIndex, rightSiblingIndex),
			'ellipsis',
			lastPageIndex,
		];
	}
	return range(1, totalPages);
};

/* -------------------------------------------------------------------------- */
/* Composable primitives                                                      */
/* -------------------------------------------------------------------------- */

export type PaginationAlign = 'start' | 'center' | 'end';

export type PaginationContainerProps = Pick<
	React.ComponentPropsWithoutRef<'nav'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/**
	 * The alignment of the pagination container.
	 * @default 'start'
	 */
	align?: PaginationAlign;
	/** The test ID to apply to the pagination container. */
	testId?: string;
};

/**
 * Root component for building custom pagination layouts.
 */
export const PaginationContainer = React.forwardRef<HTMLElement, PaginationContainerProps>(
	({ className, testId, align = 'start', ...props }, ref) => (
		<nav
			ref={ref}
			data-testid={testId}
			aria-label="pagination"
			data-slot="pagination"
			data-align={align}
			className={cn('pagination-root', className)}
			{...props}
		/>
	)
);
PaginationContainer.displayName = 'PaginationContainer';

export type PaginationContentProps = Pick<
	React.ComponentPropsWithoutRef<'ul'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The test ID to apply to the pagination content. */
	testId?: string;
};

/** Wrapper for the list of pagination items. */
export const PaginationContent = React.forwardRef<HTMLUListElement, PaginationContentProps>(
	({ className, testId, ...props }, ref) => (
		<ul
			ref={ref}
			data-testid={testId}
			data-slot="pagination-content"
			className={cn('pagination-content', className)}
			{...props}
		/>
	)
);
PaginationContent.displayName = 'PaginationContent';

export type PaginationItemProps = Pick<
	React.ComponentPropsWithoutRef<'li'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The test ID to apply to the pagination item. */
	testId?: string;
};

/** Wraps each pagination control (link, previous/next button, or ellipsis). */
export const PaginationItem = React.forwardRef<HTMLLIElement, PaginationItemProps>(
	({ className, testId, ...props }, ref) => (
		<li
			ref={ref}
			data-testid={testId}
			data-slot="pagination-item"
			className={cn('pagination-item', className)}
			{...props}
		/>
	)
);
PaginationItem.displayName = 'PaginationItem';

export type PaginationLinkProps = {
	/**
	 * If the link is active, the button will be styled as a solid button.
	 * If the link is not active, the button will be styled as a ghost button.
	 */
	isActive?: boolean;
	/** The test ID to apply to the pagination link. */
	testId?: string;
	/** Sizing preset for the button. @default 'icon' */
	size?: 'icon' | 'default';
} & Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'color'>;

/** Button for a specific page number. Set `isActive` when it is the current page. */
export const PaginationLink = React.forwardRef<HTMLButtonElement, PaginationLinkProps>(
	({ className, testId, isActive, size = 'icon', disabled, children, ...props }, ref) => (
		<button
			ref={ref}
			type="button"
			data-testid={testId}
			aria-current={isActive ? 'page' : undefined}
			tabIndex={disabled ? -1 : undefined}
			data-slot="pagination-link"
			data-active={isActive || undefined}
			data-size={size}
			className={cn('pagination-link', className)}
			disabled={disabled}
			{...props}
		>
			{children}
		</button>
	)
);
PaginationLink.displayName = 'PaginationLink';

export type PaginationNavProps = Omit<PaginationLinkProps, 'children' | 'isActive'>;

/** Button to navigate to the previous page. Disable when on the first page. */
export const PaginationPrevious = React.forwardRef<HTMLButtonElement, PaginationNavProps>(
	({ className, testId, disabled, size = 'icon', ...props }, ref) => (
		<PaginationLink
			ref={ref}
			testId={testId}
			aria-label="Go to previous page"
			size={size}
			className={cn(className)}
			disabled={disabled}
			{...props}
		>
			<ChevronLeft data-slot="pagination-nav-icon" className="pagination-nav-icon" size={16} />
		</PaginationLink>
	)
);
PaginationPrevious.displayName = 'PaginationPrevious';

/** Button to navigate to the next page. Disable when on the last page. */
export const PaginationNext = React.forwardRef<HTMLButtonElement, PaginationNavProps>(
	({ className, testId, disabled, size, ...props }, ref) => (
		<PaginationLink
			ref={ref}
			testId={testId}
			aria-label="Go to next page"
			size={size}
			className={cn(className)}
			disabled={disabled}
			{...props}
		>
			<ChevronRight data-slot="pagination-nav-icon" className="pagination-nav-icon" size={16} />
		</PaginationLink>
	)
);
PaginationNext.displayName = 'PaginationNext';

export type PaginationEllipsisProps = Pick<
	React.ComponentPropsWithoutRef<'span'>,
	'id' | 'className' | 'style' | 'children'
> & {
	/** The test ID to apply to the pagination ellipsis. */
	testId?: string;
};

/** Placeholder for omitted page numbers when there are many pages. */
export const PaginationEllipsis = React.forwardRef<HTMLSpanElement, PaginationEllipsisProps>(
	({ className, testId, ...props }, ref) => (
		<span
			ref={ref}
			data-testid={testId}
			aria-hidden
			data-slot="pagination-ellipsis"
			className={cn('pagination-ellipsis', className)}
			{...props}
		>
			<Minus height="100%" width={32} />{' '}
			<span data-slot="pagination-sr-only" className="pagination-sr-only">
				More pages
			</span>
		</span>
	)
);
PaginationEllipsis.displayName = 'PaginationEllipsis';

/* -------------------------------------------------------------------------- */
/* All-in-one Pagination                                                      */
/* -------------------------------------------------------------------------- */

export type PaginationProps = PaginationContainerProps & {
	/** The total number of items. */
	total: number;
	/**
	 * The number of items per page.
	 * @default 10
	 */
	pageSize?: number;
	/** The current page. */
	current?: number;
	/**
	 * The default current page.
	 * @default 1
	 */
	defaultCurrent?: number;
	/** The function to call when the page changes. */
	onPageChange?: (page: number) => void;
};

/**
 * All-in-one pagination component that renders previous/next buttons and page
 * numbers.
 */
export const Pagination = React.forwardRef<HTMLElement, PaginationProps>(
	(
		{ total, pageSize = 10, current: controlledCurrent, defaultCurrent = 1, onPageChange, className, align = 'start', testId, ...props },
		ref
	) => {
		const totalPages = Math.ceil(total / pageSize);
		const [internalCurrent, setInternalCurrent] = React.useState(
			controlledCurrent ?? defaultCurrent
		);
		React.useEffect(() => {
			if (controlledCurrent !== undefined) setInternalCurrent(controlledCurrent);
		}, [controlledCurrent]);
		const current = controlledCurrent ?? internalCurrent;

		const handlePageChange = (e: React.MouseEvent, page: number): void => {
			e.preventDefault();
			if (page < 1 || page > totalPages || page === current) return;
			if (onPageChange) onPageChange(page);
			else setInternalCurrent(page);
		};

		const pageNumbers = renderPageNumbers(totalPages, current);
		if (totalPages <= 1) return null;

		return (
			<PaginationContainer ref={ref} className={className} align={align} testId={testId} {...props}>
				<PaginationContent>
					<PaginationItem>
						<PaginationPrevious
							onClick={(e): void => handlePageChange(e, current - 1)}
							disabled={current === 1}
						/>
					</PaginationItem>
					{pageNumbers.map((page, idx) => (
						<PaginationItem key={page === 'ellipsis' ? `ellipsis-${idx}` : page}>
							{page === 'ellipsis' ? (
								<PaginationEllipsis />
							) : (
								<PaginationLink
									onClick={(e): void => handlePageChange(e, page)}
									isActive={page === current}
								>
									{page}
								</PaginationLink>
							)}
						</PaginationItem>
					))}
					<PaginationItem>
						<PaginationNext
							onClick={(e): void => handlePageChange(e, current + 1)}
							disabled={current === totalPages}
						/>
					</PaginationItem>
				</PaginationContent>
			</PaginationContainer>
		);
	}
);
Pagination.displayName = 'Pagination';
