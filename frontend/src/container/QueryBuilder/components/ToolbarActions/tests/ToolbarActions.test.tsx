import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ExplorerViews } from 'pages/LogsExplorer/utils';
import MockQueryClientProvider from 'providers/test/MockQueryClientProvider';

import LeftToolbarActions from '../LeftToolbarActions';
import RightToolbarActions from '../RightToolbarActions';

describe('ToolbarActions', () => {
	const mockHandleFilterVisibilityChange = (): void => {};

	const defaultItems = {
		list: {
			name: 'list',
			label: 'List View',
			disabled: false,
			show: true,
			key: ExplorerViews.LIST,
		},
		timeseries: {
			name: 'timeseries',
			label: 'Time Series',
			disabled: false,
			show: true,
			key: ExplorerViews.TIMESERIES,
		},
		datastore: {
			name: 'datastore',
			label: 'Datastore',
			disabled: false,
			show: false,
			key: 'datastore',
		},
	};

	it('LeftToolbarActions - renders correctly with default props', async () => {
		const handleChangeSelectedView = jest.fn();
		const { queryByTestId } = render(
			<LeftToolbarActions
				items={defaultItems}
				selectedView={ExplorerViews.LIST}
				onChangeSelectedView={handleChangeSelectedView}
				showFilter
				handleFilterVisibilityChange={mockHandleFilterVisibilityChange}
			/>,
		);
		expect(screen.getByTestId('search-view')).toBeInTheDocument();
		expect(screen.getByTestId('query-builder-view')).toBeInTheDocument();

		// datastore should not be present as its show: false
		expect(queryByTestId('datastore-view')).not.toBeInTheDocument();

		await userEvent.click(screen.getByTestId('search-view'));
		expect(handleChangeSelectedView).toHaveBeenCalled();

		await userEvent.click(screen.getByTestId('query-builder-view'));
		expect(handleChangeSelectedView).toHaveBeenCalled();
	});

	it('renders - datastore view and test view switching', async () => {
		const handleChangeSelectedView = jest.fn();
		const datastoreItems = {
			...defaultItems,
			list: { ...defaultItems.list, show: false },
			datastore: { ...defaultItems.datastore, show: true },
		};
		const { queryByTestId } = render(
			<LeftToolbarActions
				items={datastoreItems}
				selectedView={ExplorerViews.TIMESERIES}
				onChangeSelectedView={handleChangeSelectedView}
				showFilter
				handleFilterVisibilityChange={mockHandleFilterVisibilityChange}
			/>,
		);

		const datastoreView = queryByTestId('datastore-view');
		expect(datastoreView).toBeInTheDocument();

		await userEvent.click(datastoreView as HTMLElement);
		expect(handleChangeSelectedView).toHaveBeenCalled();

		// Test that timeseries view is also present and clickable
		const timeseriesView = queryByTestId('query-builder-view');
		expect(timeseriesView).toBeInTheDocument();

		await userEvent.click(timeseriesView as HTMLElement);
		expect(handleChangeSelectedView).toHaveBeenCalled();
	});

	it('RightToolbarActions - render correctly with props', async () => {
		const onStageRunQuery = jest.fn();
		const { queryByText } = render(
			<MockQueryClientProvider>
				<RightToolbarActions onStageRunQuery={onStageRunQuery} />,
			</MockQueryClientProvider>,
		);

		const runQueryBtn = queryByText('Run Query');
		expect(runQueryBtn).toBeInTheDocument();
		await userEvent.click(runQueryBtn as HTMLElement);
		expect(onStageRunQuery).toHaveBeenCalled();
	});
});
