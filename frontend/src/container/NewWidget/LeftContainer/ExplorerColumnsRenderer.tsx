/* eslint-disable sonarjs/cognitive-complexity */
import { useEffect, useState, type JSX } from 'react';
import {
	closestCenter,
	DndContext,
	DragEndEvent,
	PointerSensor,
	useSensor,
	useSensors,
} from '@dnd-kit/core';
import {
	arrayMove,
	horizontalListSortingStrategy,
	SortableContext,
} from '@dnd-kit/sortable';
import { Button, Input, Tooltip } from 'antd';
import { Color } from 'constants/designTokens';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuTrigger,
} from 'components/ui/dropdown-menu';
import { Divider } from 'components/ui/divider';
import { Typography } from 'components/ui/typography';
import { FieldDataType } from 'api/v5/v5';
import { SOMETHING_WENT_WRONG } from 'constants/api';
import { useQueryBuilder } from 'hooks/queryBuilder/useQueryBuilder';
import { useGetQueryKeySuggestions } from 'hooks/querySuggestions/useGetQueryKeySuggestions';
import { useIsDarkMode } from 'hooks/useDarkMode';
import useDebouncedFn from 'hooks/useDebouncedFunction';
import { CircleAlert, CirclePlus, Search } from 'components/ui/icons';
import { DataSource } from 'types/common/queryBuilder';

import { WidgetGraphProps } from '../types';
import ExplorerAttributeColumns from './ExplorerAttributeColumns';
import ExplorerColumnCard from './ExplorerColumnCard';

import './ExplorerColumnsRenderer.styles.scss';

type LogColumnsRendererProps = {
	setSelectedLogFields: WidgetGraphProps['setSelectedLogFields'];
	selectedLogFields: WidgetGraphProps['selectedLogFields'];
	selectedTracesFields: WidgetGraphProps['selectedTracesFields'];
	setSelectedTracesFields: WidgetGraphProps['setSelectedTracesFields'];
};

// Trace fields have historically arrived under either shape; the column is
// identified by whichever is present, and that name is also its sortable id.
const columnName = (field: { name?: string; key?: string }): string =>
	field?.name || field?.key || '';

function ExplorerColumnsRenderer({
	selectedLogFields,
	setSelectedLogFields,
	selectedTracesFields,
	setSelectedTracesFields,
}: LogColumnsRendererProps): JSX.Element {
	const { currentQuery } = useQueryBuilder();
	const [searchText, setSearchText] = useState<string>('');
	const [querySearchText, setQuerySearchText] = useState<string>('');
	const [open, setOpen] = useState<boolean>(false);

	const initialDataSource = currentQuery.builder.queryData[0].dataSource;

	// const { data, isLoading, isError } = useGetAggregateKeys(
	// 	{
	// 		aggregateAttribute: '',
	// 		dataSource: currentQuery.builder.queryData[0].dataSource,
	// 		aggregateOperator: currentQuery.builder.queryData[0].aggregateOperator,
	// 		searchText: querySearchText,
	// 		tagType: '',
	// 	},
	// 	{
	// 		queryKey: [
	// 			currentQuery.builder.queryData[0].dataSource,
	// 			currentQuery.builder.queryData[0].aggregateOperator,
	// 			querySearchText,
	// 		],
	// 	},
	// );

	const { data, isLoading, isError } = useGetQueryKeySuggestions(
		{
			searchText: querySearchText,
			signal: currentQuery.builder.queryData[0].dataSource,
		},
		{
			queryKey: [
				currentQuery.builder.queryData[0].dataSource,
				currentQuery.builder.queryData[0].aggregateOperator,
				querySearchText,
			],
		},
	);

	const isAttributeKeySelected = (key: string): boolean => {
		if (initialDataSource === DataSource.LOGS && selectedLogFields) {
			return selectedLogFields.some((field) => field.name === key);
		}
		if (initialDataSource === DataSource.TRACES && selectedTracesFields) {
			return selectedTracesFields.some((field) => field.name === key);
		}
		return false;
	};

	const handleCheckboxChange = (key: string): void => {
		if (
			initialDataSource === DataSource.LOGS &&
			setSelectedLogFields !== undefined
		) {
			if (selectedLogFields) {
				if (isAttributeKeySelected(key)) {
					setSelectedLogFields(
						selectedLogFields.filter((field) => field.name !== key),
					);
				} else {
					setSelectedLogFields([
						...selectedLogFields,
						{ dataType: 'string', name: key, type: '' },
					]);
				}
			} else {
				setSelectedLogFields([{ dataType: 'string', name: key, type: '' }]);
			}
		} else if (
			initialDataSource === DataSource.TRACES &&
			setSelectedTracesFields !== undefined
		) {
			const selectedField = Object.values(data?.data?.data?.keys || {})
				?.flat()
				?.find((attributeKey) => attributeKey.name === key);

			if (selectedTracesFields) {
				if (isAttributeKeySelected(key)) {
					setSelectedTracesFields(
						selectedTracesFields.filter((field) => field.name !== key),
					);
				} else if (selectedField) {
					setSelectedTracesFields([
						...selectedTracesFields,
						{
							...selectedField,
							fieldDataType: selectedField.fieldDataType as FieldDataType,
						},
					]);
				}
			} else if (selectedField) {
				setSelectedTracesFields([
					{
						...selectedField,
						fieldDataType: selectedField.fieldDataType as FieldDataType,
					},
				]);
			}
		}
		setOpen(false);
	};

	const debouncedSetQuerySearchText = useDebouncedFn((value) => {
		setQuerySearchText(value as string);
	}, 400);

	useEffect(
		() => (): void => {
			debouncedSetQuerySearchText.cancel();
		},
		[debouncedSetQuerySearchText],
	);

	const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
		setSearchText(e.target.value);
		debouncedSetQuerySearchText(e.target.value);
	};

	const handleOpenChange = (nextOpen: boolean): void => {
		setOpen(nextOpen);
		if (nextOpen) {
			setSearchText('');
		}
	};

	const removeSelectedLogField = (name: string): void => {
		if (
			initialDataSource === DataSource.LOGS &&
			setSelectedLogFields &&
			selectedLogFields
		) {
			setSelectedLogFields(
				selectedLogFields.filter((field) => field.name !== name),
			);
		}
		if (
			initialDataSource === DataSource.TRACES &&
			setSelectedTracesFields &&
			selectedTracesFields
		) {
			setSelectedTracesFields(
				selectedTracesFields.filter((field) => field.name !== name),
			);
		}
	};

	const onDragEnd = ({ active, over }: DragEndEvent): void => {
		if (!over || active.id === over.id) {
			return;
		}

		if (
			initialDataSource === DataSource.LOGS &&
			selectedLogFields &&
			setSelectedLogFields
		) {
			const oldIndex = selectedLogFields.findIndex((f) => f.name === active.id);
			const newIndex = selectedLogFields.findIndex((f) => f.name === over.id);
			setSelectedLogFields(arrayMove(selectedLogFields, oldIndex, newIndex));
		}
		if (
			initialDataSource === DataSource.TRACES &&
			selectedTracesFields &&
			setSelectedTracesFields
		) {
			const oldIndex = selectedTracesFields.findIndex(
				(f) => columnName(f) === active.id,
			);
			const newIndex = selectedTracesFields.findIndex(
				(f) => columnName(f) === over.id,
			);
			setSelectedTracesFields(arrayMove(selectedTracesFields, oldIndex, newIndex));
		}
	};

	const isDarkMode = useIsDarkMode();

	const sensors = useSensors(useSensor(PointerSensor));

	const sortableColumnNames =
		(initialDataSource === DataSource.LOGS
			? selectedLogFields?.map((field) => field.name)
			: selectedTracesFields?.map(columnName)) ?? [];

	return (
		<div className="explorer-columns-renderer">
			<div className="title">
				<Typography.Text>Columns</Typography.Text>
				{isError && (
					<Tooltip title={SOMETHING_WENT_WRONG}>
						<CircleAlert size={16} data-testid="alert-circle-icon" />
					</Tooltip>
				)}
			</div>
			<Divider className="explorer-columns-renderer__divider" />
			{!isError && (
				<div className="explorer-columns-contents">
					<DndContext
						sensors={sensors}
						collisionDetection={closestCenter}
						onDragEnd={onDragEnd}
					>
						<SortableContext
							items={sortableColumnNames}
							strategy={horizontalListSortingStrategy}
						>
							<div className="explorer-columns">
								{sortableColumnNames.map((name) => (
									<ExplorerColumnCard
										key={name}
										name={name}
										onRemove={removeSelectedLogField}
									/>
								))}
							</div>
						</SortableContext>
					</DndContext>
					<div>
						<DropdownMenu open={open} onOpenChange={handleOpenChange}>
							<DropdownMenuTrigger asChild>
								<Button
									className="action-btn"
									data-testid="add-columns-button"
									icon={
										<CirclePlus
											size={16}
											color={isDarkMode ? Color.BG_INK_400 : Color.BG_VANILLA_100}
										/>
									}
								/>
							</DropdownMenuTrigger>
							<DropdownMenuContent side="top" className="explorer-columns-dropdown">
								<Input
									type="text"
									placeholder="Search"
									className="explorer-columns-search"
									value={searchText}
									onChange={handleSearchChange}
									prefix={<Search size={16} style={{ padding: '6px' }} />}
								/>
								<ExplorerAttributeColumns
									isLoading={isLoading}
									data={data}
									searchText={searchText}
									isAttributeKeySelected={isAttributeKeySelected}
									handleCheckboxChange={handleCheckboxChange}
									dataSource={initialDataSource}
								/>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</div>
			)}
		</div>
	);
}

export default ExplorerColumnsRenderer;
