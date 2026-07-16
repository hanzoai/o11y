import * as React from 'react';

import {
	DropdownMenu,
	type DropdownMenuContentProps,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuLoading,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuSeparator,
	DropdownMenuShortcut,
	DropdownMenuSub,
	DropdownMenuSubContent,
	DropdownMenuSubTrigger,
	DropdownMenuTrigger,
} from './dropdown-menu';
import { DropdownMenuSearch } from './dropdown-menu-search';
import type {
	BaseMenuItem,
	MenuDivider,
	MenuGroup,
	MenuItem,
	MenuProps,
	RadioGroupMenuItem,
	SubMenuItem,
} from './types';

export type DropdownProps = Omit<DropdownMenuContentProps, 'children'> & {
	/** The menu configuration including items, search, and loading state. */
	menu: MenuProps;
	/** The trigger element that opens the dropdown menu. */
	children: React.ReactNode;
};

function isDivider(item: MenuItem): item is MenuDivider {
	return 'type' in item && item.type === 'divider';
}

function isGroup(item: MenuItem): item is MenuGroup {
	return 'type' in item && item.type === 'group';
}

function isSubmenu(item: MenuItem): item is SubMenuItem & { type?: never } {
	return 'children' in item && Boolean((item as SubMenuItem).children?.length);
}

/**
 * Drops empty groups, and leading, trailing, and consecutive dividers so the rendered
 * menu never shows a dangling separator.
 */
function cleanupMenuItems(items: MenuItem[]): MenuItem[] {
	const withoutEmptyGroups = items.filter((item) => {
		if (isGroup(item)) {
			return item.children && item.children.length > 0;
		}
		return true;
	});

	const cleaned: MenuItem[] = [];
	for (let i = 0; i < withoutEmptyGroups.length; i++) {
		const item = withoutEmptyGroups[i];
		if (isDivider(item)) {
			if (cleaned.length === 0) {
				continue;
			}
			if (cleaned.length > 0 && isDivider(cleaned[cleaned.length - 1])) {
				continue;
			}
			if (i === withoutEmptyGroups.length - 1) {
				continue;
			}
		}
		cleaned.push(item);
	}
	if (cleaned.length > 0 && isDivider(cleaned[cleaned.length - 1])) {
		cleaned.pop();
	}
	return cleaned;
}

function renderMenuItems(
	items: MenuItem[],
	keyPath: string[] = [],
): React.ReactNode[] {
	return items.map((item, index) => {
		const itemKey = ('key' in item && item.key) || `item-${index}`;
		const currentKeyPath = [...keyPath, itemKey];

		if (isDivider(item)) {
			return <DropdownMenuSeparator key={itemKey} />;
		}

		if (isGroup(item)) {
			return (
				<React.Fragment key={itemKey}>
					<DropdownMenuLabel>{item.label}</DropdownMenuLabel>
					{renderMenuItems(item.children, currentKeyPath)}
				</React.Fragment>
			);
		}

		if ('type' in item && item.type === 'checkbox') {
			return (
				<DropdownMenuCheckboxItem
					key={itemKey}
					checked={item.checked}
					onCheckedChange={item.onCheckedChange}
					className={item.className}
				>
					{item.label}
				</DropdownMenuCheckboxItem>
			);
		}

		if ('type' in item && item.type === 'radio-group') {
			const radioGroup = item as RadioGroupMenuItem;
			return (
				<DropdownMenuRadioGroup
					key={itemKey}
					value={radioGroup.value}
					onValueChange={radioGroup.onChange}
				>
					{radioGroup.children.map((radioItem) => (
						<DropdownMenuRadioItem
							key={radioItem.key}
							value={radioItem.value}
							disabled={radioItem.disabled}
							className={radioItem.className}
						>
							{radioItem.label}
						</DropdownMenuRadioItem>
					))}
				</DropdownMenuRadioGroup>
			);
		}

		if (isSubmenu(item)) {
			return (
				<DropdownMenuSub key={itemKey}>
					<DropdownMenuSubTrigger
						leftIcon={item.icon}
						disabled={item.disabled}
						className={item.className}
					>
						{item.label}
					</DropdownMenuSubTrigger>
					<DropdownMenuSubContent>
						{renderMenuItems(item.children, currentKeyPath)}
					</DropdownMenuSubContent>
				</DropdownMenuSub>
			);
		}

		const baseItem = item as BaseMenuItem;
		const handleSelect = () => {
			baseItem.onClick?.({ key: itemKey, keyPath: currentKeyPath });
		};
		return (
			<DropdownMenuItem
				key={itemKey}
				leftIcon={baseItem.icon}
				rightIcon={baseItem.rightIcon}
				destructive={baseItem.danger}
				disabled={baseItem.disabled}
				clickable={Boolean(baseItem.onClick)}
				onSelect={handleSelect}
				className={baseItem.className}
			>
				{baseItem.label}
				{baseItem.shortcut && (
					<DropdownMenuShortcut>{baseItem.shortcut}</DropdownMenuShortcut>
				)}
			</DropdownMenuItem>
		);
	});
}

const MENU_ITEM_SELECTOR =
	'[data-slot="dropdown-menu-item"]:not([data-disabled]), [data-slot="dropdown-menu-checkbox-item"]:not([data-disabled]), [data-slot="dropdown-menu-radio-item"]:not([data-disabled])';

/**
 * A simplified dropdown menu component with an Ant Design-style API. Renders a complete
 * dropdown menu from a declarative `menu` configuration.
 */
const DropdownMenuSimple = React.forwardRef<HTMLDivElement, DropdownProps>(
	(
		{ menu, children, sideOffset = 4, className, onOpenAutoFocus, ...props },
		ref,
	) => {
		const searchInputRef = React.useRef<HTMLInputElement>(null);
		const contentRef = React.useRef<HTMLDivElement | null>(null);

		const cleanedItems = React.useMemo(
			() => cleanupMenuItems(menu.items),
			[menu.items],
		);
		const menuItems = React.useMemo(
			() => renderMenuItems(cleanedItems),
			[cleanedItems],
		);

		const handleOpenAutoFocus = React.useCallback(
			(event: Event) => {
				if (menu.search) {
					event.preventDefault();
					searchInputRef.current?.focus();
				}
				onOpenAutoFocus?.(event);
			},
			[menu.search, onOpenAutoFocus],
		);

		const handleNavigateDown = React.useCallback(() => {
			const content = contentRef.current;
			if (!content) {
				return;
			}
			content.querySelector<HTMLElement>(MENU_ITEM_SELECTOR)?.focus();
		}, []);

		const handleContentKeyDown = React.useCallback(
			(e: React.KeyboardEvent<HTMLDivElement>) => {
				const content = contentRef.current;
				if (!content) {
					return;
				}
				const items = Array.from(
					content.querySelectorAll<HTMLElement>(MENU_ITEM_SELECTOR),
				);
				const activeElement = document.activeElement as HTMLElement | null;
				const currentIndex = activeElement ? items.indexOf(activeElement) : -1;

				if (
					(e.key === 'ArrowUp' || (e.key === 'Tab' && e.shiftKey)) &&
					menu.search
				) {
					if (activeElement === items[0]) {
						e.preventDefault();
						searchInputRef.current?.focus();
						return;
					}
				}
				if (e.target === searchInputRef.current) {
					return;
				}

				if (e.key === 'Tab' && !e.shiftKey && currentIndex !== -1) {
					e.preventDefault();
					const nextIndex = currentIndex + 1;
					if (nextIndex < items.length) {
						items[nextIndex].focus();
					} else if (nextIndex === items.length && searchInputRef.current) {
						searchInputRef.current.focus();
					} else if (
						nextIndex === items.length &&
						!searchInputRef.current &&
						items.length > 0
					) {
						items[0].focus();
					}
					return;
				}
				if (e.key === 'Tab' && e.shiftKey && currentIndex !== -1) {
					e.preventDefault();
					const prevIndex = currentIndex - 1;
					if (prevIndex >= 0) {
						items[prevIndex].focus();
					} else if (prevIndex === -1 && searchInputRef.current) {
						searchInputRef.current.focus();
					} else if (
						prevIndex === -1 &&
						!searchInputRef.current &&
						items.length > 0
					) {
						items[items.length - 1].focus();
					}
				}
			},
			[menu.search],
		);

		const mergedContentRef = React.useCallback(
			(node: HTMLDivElement | null) => {
				contentRef.current = node;
				if (typeof ref === 'function') {
					ref(node);
				} else if (ref) {
					ref.current = node;
				}
			},
			[ref],
		);

		const handleOpenChange = React.useCallback(
			(open: boolean) => {
				if (open && menu.search) {
					searchInputRef.current?.focus();
				} else if (!open) {
					menu.search?.onSearchChange?.('');
				}
			},
			[menu.search],
		);

		return (
			<DropdownMenu onOpenChange={handleOpenChange}>
				<DropdownMenuTrigger asChild>{children}</DropdownMenuTrigger>
				<DropdownMenuContent
					ref={mergedContentRef}
					sideOffset={sideOffset}
					className={className}
					onOpenAutoFocus={handleOpenAutoFocus}
					onKeyDown={handleContentKeyDown}
					{...props}
				>
					{menu.search && (
						<>
							<DropdownMenuSearch
								ref={searchInputRef}
								placeholder={menu.search.placeholder}
								searchIcon={menu.search.searchIcon}
								onSearchChange={menu.search.onSearchChange}
								onNavigateDown={handleNavigateDown}
							/>
							<DropdownMenuSeparator />
						</>
					)}
					{menu.loading ? (
						<DropdownMenuLoading
							text={typeof menu.loading === 'object' ? menu.loading.text : undefined}
						/>
					) : (
						menuItems
					)}
				</DropdownMenuContent>
			</DropdownMenu>
		);
	},
);
DropdownMenuSimple.displayName = 'DropdownMenuSimple';

export { DropdownMenuSimple };
