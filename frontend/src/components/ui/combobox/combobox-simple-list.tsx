import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import { Fragment, type ReactNode } from 'react';
import {
	ComboboxCreateItem,
	ComboboxEmpty,
	ComboboxGroup,
	ComboboxHint,
	ComboboxItem,
	ComboboxLoading,
	ComboboxPill,
	ComboboxSeparator,
} from './combobox';
import type { ComboboxSimpleGroup, ComboboxSimpleItem } from './types';

type AllowCreate = boolean | ((inputValue: string) => ReactNode);

export type ComboboxListParams = {
	loading: boolean;
	loadingPlaceholder: ReactNode;
	groups?: ComboboxSimpleGroup[];
	items: ComboboxSimpleItem[];
	selectedValues: string[];
	onSelect: (value: string) => void;
	onInsert: (value: string) => void;
	onCreate: (value: string) => void;
	emptyPlaceholder: ReactNode;
	showCreateOption: boolean;
	inputValue: string;
	allowCreate: AllowCreate;
	customValues: string[];
	filterHints: (items: ComboboxSimpleItem[]) => ComboboxSimpleItem[];
};

export type ComboboxMultiPillsParams = {
	selectedValues: string[];
	maxDisplayedPills?: number;
	resolveLabel: (value: string) => ReactNode;
	onRemove: (value: string) => void;
};

function renderItem(
	item: ComboboxSimpleItem,
	selectedValues: string[],
	onSelect: (value: string) => void,
	onInsert: (value: string) => void
): ReactNode {
	if (item.insertValue !== undefined) {
		return (
			<ComboboxHint
				key={item.value}
				value={item.value}
				insertValue={item.insertValue}
				onInsert={onInsert}
			>
				{item.label}
			</ComboboxHint>
		);
	}
	return (
		<ComboboxItem
			key={item.value}
			value={item.value}
			onSelect={onSelect}
			isSelected={selectedValues.includes(item.value)}
		>
			{item.label}
		</ComboboxItem>
	);
}

function renderCustomGroup(customValues: string[], onSelect: (value: string) => void): ReactNode {
	return (
		<ComboboxGroup heading="Custom" forceMount>
			{customValues.map((v) => (
				<ComboboxItem key={v} value={v} onSelect={onSelect} isSelected forceMount>
					{v}
				</ComboboxItem>
			))}
		</ComboboxGroup>
	);
}

function renderCreateOption(p: ComboboxListParams): ReactNode {
	if (!p.showCreateOption) {
		return null;
	}
	const trimmed = p.inputValue.trim();
	return (
		<ComboboxCreateItem
			inputValue={trimmed}
			value={`__create__${trimmed}`}
			onSelect={(): void => p.onCreate(p.inputValue)}
		>
			{typeof p.allowCreate === 'function' ? p.allowCreate(trimmed) : `Create "${trimmed}"`}
		</ComboboxCreateItem>
	);
}

function renderEmpty(p: ComboboxListParams): ReactNode {
	if (p.showCreateOption || p.customValues.length > 0) {
		return null;
	}
	return <ComboboxEmpty>{p.emptyPlaceholder}</ComboboxEmpty>;
}

function renderGroups(p: ComboboxListParams, groups: ComboboxSimpleGroup[]): ReactNode {
	return (
		<>
			{p.customValues.length > 0 && (
				<>
					{renderCustomGroup(p.customValues, p.onSelect)}
					<ComboboxSeparator />
				</>
			)}
			{groups.map((group, idx) => {
				const filtered = p.filterHints(group.items);
				if (filtered.length === 0) {
					return null;
				}
				return (
					<Fragment key={group.heading ?? idx}>
						{idx > 0 && <ComboboxSeparator />}
						<ComboboxGroup heading={group.heading}>
							{filtered.map((item) =>
								renderItem(item, p.selectedValues, p.onSelect, p.onInsert)
							)}
						</ComboboxGroup>
					</Fragment>
				);
			})}
			{renderCreateOption(p)}
			{renderEmpty(p)}
		</>
	);
}

function renderFlat(p: ComboboxListParams): ReactNode {
	return (
		<>
			{p.customValues.length > 0 && (
				<>
					{renderCustomGroup(p.customValues, p.onSelect)}
					{p.items.length > 0 && <ComboboxSeparator />}
				</>
			)}
			{p.filterHints(p.items).map((item) =>
				renderItem(item, p.selectedValues, p.onSelect, p.onInsert)
			)}
			{renderCreateOption(p)}
			{renderEmpty(p)}
		</>
	);
}

/**
 * Renders the option list inside the combobox popover: loading, grouped, or flat.
 */
export function renderComboboxList(p: ComboboxListParams): ReactNode {
	if (p.loading) {
		return <ComboboxLoading>{p.loadingPlaceholder}</ComboboxLoading>;
	}
	if (p.groups) {
		return renderGroups(p, p.groups);
	}
	return renderFlat(p);
}

/**
 * Renders the selected pills (with a "+N" overflow tooltip) for multi-select mode.
 */
export function renderComboboxMultiPills(p: ComboboxMultiPillsParams): ReactNode {
	if (p.selectedValues.length === 0) {
		return undefined;
	}
	const displayed =
		p.maxDisplayedPills !== undefined
			? p.selectedValues.slice(0, p.maxDisplayedPills)
			: p.selectedValues;
	const overflowCount =
		p.maxDisplayedPills !== undefined
			? Math.max(0, p.selectedValues.length - p.maxDisplayedPills)
			: 0;
	const hidden = p.selectedValues.slice(p.maxDisplayedPills);
	const overflowTitle = hidden
		.map((v) => {
			const label = p.resolveLabel(v);
			return typeof label === 'string' ? label : v;
		})
		.join(', ');

	return (
		<span data-slot="combobox-pills" className="flex flex-wrap items-center gap-1">
			{displayed.map((v) => (
				<ComboboxPill key={v} value={v} onRemove={p.onRemove}>
					{p.resolveLabel(v)}
				</ComboboxPill>
			))}
			{overflowCount > 0 && (
				<TooltipPrimitive.Root>
					<TooltipPrimitive.Trigger asChild>
						<span
							data-slot="combobox-pill-overflow"
							className="inline-flex h-5 shrink-0 cursor-default items-center justify-center rounded-[2px] bg-[var(--muted)] px-2 text-xs font-medium leading-none text-[var(--muted-foreground)]"
						>
							+{overflowCount}
						</span>
					</TooltipPrimitive.Trigger>
					<TooltipPrimitive.Portal>
						<TooltipPrimitive.Content
							sideOffset={4}
							className="z-50 max-w-xs rounded-[2px] border border-[var(--border)] bg-[var(--popover)] px-2 py-1 text-xs text-[var(--popover-foreground)] shadow-md"
						>
							{overflowTitle}
						</TooltipPrimitive.Content>
					</TooltipPrimitive.Portal>
				</TooltipPrimitive.Root>
			)}
		</span>
	);
}
