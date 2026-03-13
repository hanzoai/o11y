import { ChangeEvent, useCallback } from 'react';
import MEditor, { Monaco } from '@monaco-editor/react';
import { Color } from 'constants/designTokens';
import { Input } from 'antd';
import { LEGEND } from 'constants/global';
import { useQueryBuilder } from 'hooks/queryBuilder/useQueryBuilder';
import { useIsDarkMode } from 'hooks/useDarkMode';
import { IDatastoreQuery } from 'types/api/queryBuilder/queryBuilderData';
import { EQueryType } from 'types/common/dashboard';
import { getFormatedLegend } from 'utils/getFormatedLegend';

import QueryHeader from '../QueryHeader';

interface IDatastoreQueryBuilderProps {
	queryData: IDatastoreQuery;
	queryIndex: number;
	deletable: boolean;
}

function DatastoreQueryBuilder({
	queryData,
	queryIndex,
	deletable,
}: IDatastoreQueryBuilderProps): JSX.Element | null {
	const {
		handleSetQueryItemData,
		removeQueryTypeItemByIndex,
	} = useQueryBuilder();

	const handleRemoveQuery = useCallback(() => {
		removeQueryTypeItemByIndex(EQueryType.DATASTORE, queryIndex);
	}, [queryIndex, removeQueryTypeItemByIndex]);

	const handleUpdateQuery = useCallback(
		<Field extends keyof IDatastoreQuery, Value extends IDatastoreQuery[Field]>(
			field: keyof IDatastoreQuery,
			value: Value,
		) => {
			const newQuery: IDatastoreQuery = { ...queryData, [field]: value };

			handleSetQueryItemData(queryIndex, EQueryType.DATASTORE, newQuery);
		},
		[handleSetQueryItemData, queryIndex, queryData],
	);

	const handleDisable = useCallback(() => {
		const newQuery: IDatastoreQuery = {
			...queryData,
			disabled: !queryData.disabled,
		};

		handleSetQueryItemData(queryIndex, EQueryType.DATASTORE, newQuery);
	}, [handleSetQueryItemData, queryData, queryIndex]);

	const handleUpdateEditor = useCallback(
		(value: string | undefined) => {
			if (value !== undefined) {
				handleUpdateQuery('query', value);
			}
		},
		[handleUpdateQuery],
	);

	const handleUpdateInput = useCallback(
		(e: ChangeEvent<HTMLInputElement>) => {
			const { name } = e.target;
			let { value } = e.target;
			if (name === LEGEND) {
				value = getFormatedLegend(value);
			}
			handleUpdateQuery(name as keyof IDatastoreQuery, value);
		},
		[handleUpdateQuery],
	);

	const isDarkMode = useIsDarkMode();

	function setEditorTheme(monaco: Monaco): void {
		monaco.editor.defineTheme('my-theme', {
			base: 'vs-dark',
			inherit: true,
			rules: [
				{ token: 'string.key.json', foreground: Color.BG_VANILLA_400 },
				{ token: 'string.value.json', foreground: Color.BG_ROBIN_400 },
			],
			colors: {
				'editor.background': Color.BG_INK_300,
			},
		});
	}

	return (
		<QueryHeader
			name={queryData?.name}
			disabled={queryData?.disabled}
			onDisable={handleDisable}
			onDelete={handleRemoveQuery}
			deletable={deletable}
		>
			<MEditor
				language="sql"
				height="200px"
				onChange={handleUpdateEditor}
				value={queryData?.query}
				onMount={(_, monaco): void => {
					document.fonts.ready.then(() => {
						monaco.editor.remeasureFonts();
					});
				}}
				options={{
					scrollbar: {
						alwaysConsumeMouseWheel: false,
					},
					minimap: {
						enabled: false,
					},
					fontSize: 14,
					fontFamily: 'Geist Mono',
				}}
				theme={isDarkMode ? 'my-theme' : 'light'}
				beforeMount={setEditorTheme}
			/>
			<Input
				onChange={handleUpdateInput}
				name="legend"
				size="middle"
				defaultValue={queryData?.legend}
				value={queryData?.legend}
				addonBefore="Legend Format"
			/>
		</QueryHeader>
	);
}

export default DatastoreQueryBuilder;
