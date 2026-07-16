import * as React from 'react';

import { Input, type InputProps } from '../input';
import { cn } from '../lib/utils';

export type DropdownMenuSearchProps = Omit<
	InputProps,
	'onChange' | 'prefix'
> & {
	/** Callback fired when the search query changes. */
	onSearchChange?: (value: string) => void;
	/** Optional icon to display before the input. */
	searchIcon?: React.ReactNode;
	/**
	 * Callback fired when ArrowDown is pressed to navigate to menu items.
	 * @internal
	 */
	onNavigateDown?: () => void;
};

const navigationKeys = ['ArrowUp', 'ArrowDown', 'Enter', 'Escape', 'Tab'];

/**
 * A search input for filtering dropdown menu items. Typically placed at the top of the
 * dropdown content.
 */
const DropdownMenuSearch = React.forwardRef<
	HTMLInputElement,
	DropdownMenuSearchProps
>(
	(
		{
			className,
			onSearchChange,
			onNavigateDown,
			searchIcon,
			placeholder = 'Search...',
			...props
		},
		ref,
	) => {
		const [value, setValue] = React.useState('');

		const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
			const newValue = e.target.value;
			setValue(newValue);
			onSearchChange?.(newValue);
		};

		const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
			if (
				(e.key === 'ArrowDown' || (e.key === 'Tab' && !e.shiftKey)) &&
				onNavigateDown
			) {
				e.preventDefault();
				onNavigateDown();
			} else if (!navigationKeys.includes(e.key)) {
				e.stopPropagation();
			}
		};

		const input = (
			<Input
				ref={ref}
				type="text"
				data-slot="dropdown-menu-search"
				value={value}
				onChange={handleChange}
				onKeyDown={handleKeyDown}
				placeholder={placeholder}
				className={cn(className)}
				{...props}
			/>
		);

		if (!searchIcon) {
			return input;
		}

		return (
			<div data-slot="dropdown-menu-search-wrapper">
				<span data-slot="dropdown-menu-search-icon">{searchIcon}</span>
				{input}
			</div>
		);
	},
);
DropdownMenuSearch.displayName = 'DropdownMenuSearch';

export { DropdownMenuSearch };
