import type { CSSProperties, ReactNode } from 'react';

export type ComboboxSimpleItem = {
	/**
	 * Unique value for the option.
	 */
	value: string;
	/**
	 * Display content for the option. Can be string or ReactNode (e.g. icon + text).
	 */
	label: ReactNode;
	/**
	 * Optional string to show in the trigger instead of label. Use when label is
	 * ReactNode but you want plain text in the trigger.
	 */
	displayValue?: string;
	/**
	 * When set, item becomes a "hint" that inserts this value into input instead of
	 * selecting. Useful for suggestions like "status:" that let users continue typing.
	 */
	insertValue?: string;
	/**
	 * Additional keywords for filtering. Useful when value differs from searchable text.
	 * E.g. value="15" with keywords=["15 minutes", "quarter hour"]
	 */
	keywords?: string[];
};

export type ComboboxSimpleGroup = {
	/**
	 * Optional heading for the group.
	 */
	heading?: string;
	/**
	 * Items in this group.
	 */
	items: ComboboxSimpleItem[];
};

export type ComboboxSimpleProps = {
	/**
	 * The testId associated with the combobox.
	 */
	testId?: string;
	/**
	 * The id of the combobox.
	 */
	id?: string;
	/**
	 * Additional CSS classes to apply to the combobox trigger.
	 */
	className?: string;
	/**
	 * Inline styles to apply to the combobox trigger.
	 */
	style?: CSSProperties;
	/**
	 * List of items to display (flat). Ignored when groups is provided.
	 * @default []
	 */
	items?: ComboboxSimpleItem[];
	/**
	 * Grouped items with optional headings. When provided, items is ignored.
	 * @default undefined
	 */
	groups?: ComboboxSimpleGroup[];
	/**
	 * Placeholder text when no value is selected.
	 * @default 'Select an option...'
	 */
	placeholder?: string;
	/**
	 * Placeholder text for the search input inside the popover.
	 * Falls back to `placeholder` if not provided.
	 * @default undefined
	 */
	inputPlaceholder?: string;
	/**
	 * Text shown when there are no results (e.g. after filtering).
	 * @default 'No results found.'
	 */
	emptyPlaceholder?: string;
	/**
	 * Controlled selected value. When `multiple` is true, this should be an array.
	 * @default undefined
	 */
	value?: string | string[];
	/**
	 * Initial value when uncontrolled. When `multiple` is true, this should be an array.
	 * @default '' (or [] when multiple)
	 */
	defaultValue?: string | string[];
	/**
	 * Callback when selection changes.
	 * @param value - The new selected value (string when single, string[] when multiple).
	 */
	onChange?: (value: string | string[]) => void;
	/**
	 * Customize what is shown in the trigger. Receives the selected item (or undefined).
	 * Use when you want a string instead of the label ReactNode.
	 */
	displayValue?: (item: ComboboxSimpleItem | undefined) => string | ReactNode;
	/**
	 * Only change to false when you want to include this component inside a popover.
	 * @default true
	 */
	withPortal?: boolean;
	/**
	 * Enable multi-select mode. Values are shown as removable pills.
	 * @default false
	 */
	multiple?: boolean;
	/**
	 * Allow creating new items by typing and pressing Enter.
	 * When `true`, shows a default "Create [input]" option.
	 * When a function, renders custom content for the create option.
	 * @default false
	 */
	allowCreate?: boolean | ((inputValue: string) => React.ReactNode);
	/**
	 * Maximum number of pills to display in multi-select mode.
	 * Overflow items are shown as "+N" badge.
	 * @default undefined (show all)
	 */
	maxDisplayedPills?: number;
	/**
	 * Disable the internal TooltipProvider wrapper.
	 * Set to true when ComboboxSimple is already inside a TooltipProvider.
	 * @default false
	 */
	disableTooltipProvider?: boolean;
	/**
	 * Show loading state instead of items.
	 * @default false
	 */
	loading?: boolean;
	/**
	 * Content shown while loading. Can be string or ReactNode.
	 * @default 'Loading...'
	 */
	loadingPlaceholder?: ReactNode;
	/**
	 * Whether the combobox is disabled.
	 * @default false
	 */
	disabled?: boolean;
};
