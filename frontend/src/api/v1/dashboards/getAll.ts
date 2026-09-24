import axios from 'api';
import { ErrorResponseHandlerV2 } from 'api/ErrorResponseHandlerV2';
import { AxiosError } from 'axios';
import { tagsToStrings } from 'pages/DashboardsListPageV2/utils';
import { ErrorV2Resp, SuccessResponseV2 } from 'types/api';
import { Dashboard } from 'types/api/dashboard/getAll';

// GET /dashboards answers the V2 list — {dashboards, total, tags}, one page at
// a time — where this reader once received a bare array. Home, the export panel,
// the assistant's picker and the legacy list all read it through here and sort
// or map what they get, so the first array method they called threw and took the
// console down with it. This reads every page and hands them the Dashboard they
// render. A list item carries no widgets, layout or variables; the screens that
// need a dashboard's body fetch it by id.
const PAGE = 200; // dashboardtypes.MaxListLimit

interface ListedDashboard {
	id: string;
	createdAt: string;
	updatedAt: string;
	createdBy: string;
	updatedBy: string;
	locked: boolean;
	name: string;
	image?: string;
	tags: { key: string; value: string }[] | null;
	spec?: { display?: { name?: string; description?: string } };
}

interface ListPage {
	status: string;
	data: { dashboards: ListedDashboard[] | null; total: number };
}

const toDashboard = (d: ListedDashboard): Dashboard => ({
	id: d.id,
	createdAt: d.createdAt,
	updatedAt: d.updatedAt,
	createdBy: d.createdBy,
	updatedBy: d.updatedBy,
	locked: d.locked,
	data: {
		title: d.spec?.display?.name || d.name,
		description: d.spec?.display?.description,
		tags: tagsToStrings(d.tags),
		image: d.image,
		variables: {},
	},
});

const getAll = async (): Promise<SuccessResponseV2<Dashboard[]>> => {
	try {
		const dashboards: Dashboard[] = [];
		let httpStatusCode = 200;
		for (let offset = 0; ; offset += PAGE) {
			const response = await axios.get<ListPage>('/dashboards', {
				params: { limit: PAGE, offset, sort: 'updated_at', order: 'desc' },
			});
			httpStatusCode = response.status;
			const listed = response.data.data.dashboards ?? [];
			dashboards.push(...listed.map(toDashboard));
			if (listed.length < PAGE || dashboards.length >= response.data.data.total) {
				break;
			}
		}
		return { httpStatusCode, data: dashboards };
	} catch (error) {
		ErrorResponseHandlerV2(error as AxiosError<ErrorV2Resp>);
	}
};

export default getAll;
