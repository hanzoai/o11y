import React from 'react';
import { ToggleGroup, ToggleGroupItem, type ToggleGroupProps } from '../toggle-group';

export type ToggleGroupSimpleItem = {
	/**
	 * Unique value for the option.
	 */
	value: string;
	/**
	 * Display content. Can be string, icon only, or ReactNode (e.g. icon + text).
	 */
	label: React.ReactNode;
	/**
	 * Optional aria-label for accessibility when label is icon-only or complex.
	 */
	'aria-label'?: string;
};

export type ToggleGroupSimpleProps = Omit<ToggleGroupProps, 'children'> & {
	/**
	 * List of items to display.
	 */
	items?: ToggleGroupSimpleItem[];
};

/**
 * Minimal toggle group preset. Accepts a list of items and renders toggle buttons
 * with minimal configuration. Supports icon-only, label-only, or icon + label via
 * label as ReactNode.
 */
const ToggleGroupSimpleInner = React.forwardRef<HTMLDivElement, ToggleGroupSimpleProps>(
	({ items = [], size = 'default', color = 'secondary', ...props }, ref) => (
		<ToggleGroup ref={ref} size={size} color={color} {...(props as unknown as ToggleGroupProps)}>
			{items.map((item) => (
				<ToggleGroupItem key={item.value} value={item.value} aria-label={item['aria-label']}>
					{item.label}
				</ToggleGroupItem>
			))}
		</ToggleGroup>
	)
);
ToggleGroupSimpleInner.displayName = 'ToggleGroupSimpleInner';

export const ToggleGroupSimple = React.memo(ToggleGroupSimpleInner);
