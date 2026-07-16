import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import { defaultFilter as commandDefaultFilter } from 'cmdk';
import { ChevronDown, LoaderCircle } from 'lucide-react';
import {
	Fragment,
	forwardRef,
	type KeyboardEvent,
	memo,
	type ReactNode,
	useCallback,
	useId,
	useMemo,
	useRef,
	useState,
} from 'react';
import { cn } from '../lib/utils';
import {
	Combobox,
	ComboboxCommand,
	ComboboxContent,
	ComboboxInput,
	ComboboxList,
	ComboboxTrigger,
} from './combobox';
import {
	renderComboboxList,
	renderComboboxMultiPills,
} from './combobox-simple-list';
import type {
	ComboboxSimpleGroup,
	ComboboxSimpleItem,
	ComboboxSimpleProps,
} from './types';

const triggerClass =
	'flex h-8 w-full items-center justify-between gap-2 whitespace-nowrap rounded-[calc(var(--radius)-2px)] border border-[var(--input)] bg-transparent px-3 py-2 text-[13px] leading-5 shadow-sm cursor-pointer focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--ring)] focus-within:ring-1 focus-within:ring-[var(--ring)] disabled:cursor-not-allowed disabled:opacity-50 data-[disabled=true]:cursor-not-allowed data-[disabled=true]:opacity-50';
const valueClass =
	'flex items-center gap-2 overflow-hidden text-[13px] leading-5 [&_svg]:size-4 [&_svg]:shrink-0';

function flattenItems(groups: ComboboxSimpleGroup[]): ComboboxSimpleItem[] {
	return groups.flatMap((g) => g.items);
}

function normalizeValue(v?: string | string[]): string[] {
	if (v === undefined) {
		return [];
	}
	return (Array.isArray(v) ? v : [v]).filter((val) => val !== '');
}

/**
 * Minimal combobox preset. Accepts a list of items and handles selection,
 * filtering, and value display with minimal configuration.
 */
const ComboboxSimpleInner = forwardRef<
	HTMLButtonElement | HTMLDivElement,
	ComboboxSimpleProps
>(
	(
		{
			items = [],
			groups,
			placeholder = 'Select an option...',
			inputPlaceholder,
			emptyPlaceholder = 'No results found.',
			value: controlledValue,
			defaultValue,
			onChange,
			displayValue: displayValueFn,
			withPortal = true,
			testId,
			id,
			className,
			style,
			multiple = false,
			allowCreate = false,
			maxDisplayedPills,
			disableTooltipProvider = false,
			loading = false,
			loadingPlaceholder = 'Loading...',
			disabled = false,
		},
		forwardedRef,
	) => {
		const listId = useId();
		const [uncontrolledValue, setUncontrolledValue] = useState<string[]>(() =>
			normalizeValue(defaultValue),
		);
		const [open, setOpenInternal] = useState(false);
		const setOpen = useCallback(
			(next: boolean) => {
				if (disabled) {
					return;
				}
				setOpenInternal(next);
			},
			[disabled],
		);
		const [inputValue, setInputValue] = useState('');
		const internalRef = useRef<HTMLButtonElement | HTMLDivElement | null>(null);
		const triggerRef = useMemo(
			() =>
				(node: HTMLButtonElement | HTMLDivElement | null): void => {
					internalRef.current = node;
					if (typeof forwardedRef === 'function') {
						forwardedRef(node);
					} else if (forwardedRef) {
						forwardedRef.current = node;
					}
				},
			[forwardedRef],
		);

		const isControlled = controlledValue !== undefined;
		const selectedValues = useMemo(
			() => (isControlled ? normalizeValue(controlledValue) : uncontrolledValue),
			[isControlled, controlledValue, uncontrolledValue],
		);

		const allItems = useMemo(
			() => (groups ? flattenItems(groups) : items),
			[groups, items],
		);
		const itemsMap = useMemo(() => {
			const map = new Map<string, ComboboxSimpleItem>();
			for (const item of allItems) {
				map.set(item.value, item);
			}
			return map;
		}, [allItems]);
		const searchStringsMap = useMemo(() => {
			const map = new Map<string, string[]>();
			for (const item of allItems) {
				const searchable: string[] = [item.value];
				if (item.displayValue) {
					searchable.push(item.displayValue);
				}
				if (item.insertValue) {
					searchable.push(item.insertValue);
				}
				if (typeof item.label === 'string') {
					searchable.push(item.label);
				}
				if (item.keywords) {
					searchable.push(...item.keywords);
				}
				map.set(
					item.value,
					searchable.map((s) => s.toLowerCase()),
				);
			}
			return map;
		}, [allItems]);

		const handleSelect = useCallback(
			(selectedValue: string) => {
				if (multiple) {
					const newValues = selectedValues.includes(selectedValue)
						? selectedValues.filter((v) => v !== selectedValue)
						: [...selectedValues, selectedValue];
					if (!isControlled) {
						setUncontrolledValue(newValues);
					}
					onChange?.(newValues);
					setInputValue('');
				} else {
					if (!isControlled) {
						setUncontrolledValue([selectedValue]);
					}
					onChange?.(selectedValue);
					setInputValue('');
					setOpen(false);
				}
			},
			[multiple, onChange, selectedValues, isControlled, setOpen],
		);

		const handleRemove = useCallback(
			(valueToRemove: string) => {
				const newValues = selectedValues.filter((v) => v !== valueToRemove);
				if (!isControlled) {
					setUncontrolledValue(newValues);
				}
				onChange?.(newValues);
			},
			[onChange, selectedValues, isControlled],
		);

		const handleCreate = useCallback(
			(valueToCreate: string) => {
				const trimmed = valueToCreate.trim();
				if (!trimmed) {
					return;
				}
				if (selectedValues.includes(trimmed)) {
					setInputValue('');
					return;
				}
				if (multiple) {
					const newValues = [...selectedValues, trimmed];
					if (!isControlled) {
						setUncontrolledValue(newValues);
					}
					onChange?.(newValues);
				} else {
					if (!isControlled) {
						setUncontrolledValue([trimmed]);
					}
					onChange?.(trimmed);
					setOpen(false);
				}
				setInputValue('');
			},
			[multiple, onChange, selectedValues, isControlled, setOpen],
		);

		const selectedItem = useMemo(
			() =>
				multiple
					? undefined
					: allItems.find((item) => item.value === selectedValues[0]),
			[multiple, allItems, selectedValues],
		);
		const singleCustomValue =
			!multiple && selectedValues.length > 0 && !selectedItem
				? selectedValues[0]
				: undefined;
		const triggerValue = displayValueFn
			? displayValueFn(selectedItem)
			: selectedItem
				? (selectedItem.displayValue ?? selectedItem.label)
				: singleCustomValue;

		const resolveLabel = useCallback(
			(value: string): ReactNode => {
				const item = itemsMap.get(value);
				if (!item) {
					return value;
				}
				return item.displayValue ?? item.label;
			},
			[itemsMap],
		);

		const handleTriggerKeyDown = useCallback(
			(e: KeyboardEvent) => {
				if (e.key === 'Enter' || e.key === ' ') {
					e.preventDefault();
					setOpen(true);
				}
			},
			[setOpen],
		);
		const handleInputKeyDown = useCallback(
			(e: KeyboardEvent<HTMLInputElement>) => {
				if (e.key === 'Tab' && e.shiftKey) {
					e.preventDefault();
					setOpen(false);
					internalRef.current?.focus();
				}
			},
			[setOpen],
		);
		const handleInsert = useCallback((value: string) => {
			setInputValue(value);
		}, []);

		const hintItems = useMemo(
			() => allItems.filter((item) => item.insertValue !== undefined),
			[allItems],
		);
		const showHints =
			hintItems.length > 0 &&
			!hintItems.some((item) => inputValue.startsWith(item.insertValue as string));
		const hintValues = useMemo(
			() => new Set(hintItems.map((h) => h.value)),
			[hintItems],
		);

		const customFilter = useCallback(
			(value: string, search: string, keywords?: string[]): number => {
				if (hintValues.has(value)) {
					return showHints ? 1 : 0;
				}
				const searchStrings = searchStringsMap.get(value);
				if (searchStrings) {
					return commandDefaultFilter(value, search, [
						...searchStrings,
						...(keywords ?? []),
					]);
				}
				return commandDefaultFilter(value, search, keywords);
			},
			[hintValues, showHints, searchStringsMap],
		);

		const trimmedInput = inputValue.trim();
		const showCreateOption =
			Boolean(allowCreate) &&
			trimmedInput.length > 0 &&
			!selectedValues.includes(trimmedInput) &&
			!allItems.some((item) => item.value === trimmedInput);
		const customValues = useMemo(
			() => selectedValues.filter((v) => !itemsMap.has(v)),
			[selectedValues, itemsMap],
		);
		const filterHints = useCallback(
			(itemList: ComboboxSimpleItem[]): ComboboxSimpleItem[] =>
				itemList.filter((item) =>
					item.insertValue !== undefined ? showHints : true,
				),
			[showHints],
		);

		const listContent = renderComboboxList({
			loading,
			loadingPlaceholder,
			groups,
			items,
			selectedValues,
			onSelect: handleSelect,
			onInsert: handleInsert,
			onCreate: handleCreate,
			emptyPlaceholder,
			showCreateOption,
			inputValue,
			allowCreate,
			customValues,
			filterHints,
		});

		const Wrapper = disableTooltipProvider ? Fragment : TooltipPrimitive.Provider;

		const commandContent = (
			<ComboboxCommand filter={customFilter}>
				<ComboboxInput
					placeholder={inputPlaceholder ?? placeholder}
					value={inputValue}
					onValueChange={setInputValue}
					onKeyDown={handleInputKeyDown}
				/>
				<ComboboxList id={listId}>{listContent}</ComboboxList>
			</ComboboxCommand>
		);

		const spinnerOrChevron = loading ? (
			<LoaderCircle
				data-slot="combobox-spinner"
				className="size-4 shrink-0 animate-spin opacity-50"
			/>
		) : (
			<ChevronDown
				data-slot="combobox-icon"
				className="size-4 shrink-0 opacity-50"
			/>
		);

		if (multiple) {
			const pillsContent = renderComboboxMultiPills({
				selectedValues,
				maxDisplayedPills,
				resolveLabel,
				onRemove: handleRemove,
			});

			return (
				<Wrapper>
					<Combobox open={open} onOpenChange={setOpen}>
						<ComboboxTrigger asChild>
							<div
								ref={triggerRef}
								className={cn(triggerClass, className)}
								style={style}
								data-slot="combobox-trigger"
								data-testid={testId}
								id={id}
								role="combobox"
								aria-controls={listId}
								aria-expanded={open}
								aria-haspopup="listbox"
								aria-disabled={disabled}
								data-disabled={disabled || undefined}
								tabIndex={disabled ? -1 : 0}
								onKeyDown={disabled ? undefined : handleTriggerKeyDown}
							>
								<span data-slot="combobox-value" className={valueClass}>
									{pillsContent || placeholder || 'Select an option...'}
								</span>
								{spinnerOrChevron}
							</div>
						</ComboboxTrigger>
						{open && (
							<ComboboxContent withPortal={withPortal}>
								{commandContent}
							</ComboboxContent>
						)}
					</Combobox>
				</Wrapper>
			);
		}

		return (
			<Wrapper>
				<Combobox open={open} onOpenChange={setOpen}>
					<ComboboxTrigger asChild>
						<button
							ref={triggerRef}
							type="button"
							className={cn(triggerClass, className)}
							style={style}
							data-slot="combobox-trigger"
							data-testid={testId}
							id={id}
							disabled={disabled}
							data-disabled={disabled || undefined}
						>
							<span data-slot="combobox-value" className={valueClass}>
								{triggerValue || placeholder || 'Select an option...'}
							</span>
							{spinnerOrChevron}
						</button>
					</ComboboxTrigger>
					{open && (
						<ComboboxContent withPortal={withPortal}>
							{commandContent}
						</ComboboxContent>
					)}
				</Combobox>
			</Wrapper>
		);
	},
);
ComboboxSimpleInner.displayName = 'ComboboxSimpleInner';

const ComboboxSimple = memo(ComboboxSimpleInner);

export { ComboboxSimple };
