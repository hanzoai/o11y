import { pipelineFields } from '../config';

import type { JSX } from 'react';

export const renderPipelineForm = (): Array<JSX.Element> =>
	pipelineFields.map((field) => {
		const Component = field.component;
		return <Component key={field.id} fieldData={field} />;
	});
