import { Input } from 'components/ui/input';
import { Form } from 'antd';
import { CloudintegrationtypesCredentialsDTO } from 'api/generated/services/o11y.schemas';

function RenderConnectionFields({
	isConnectionParamsLoading,
	connectionParams,
	isFormDisabled,
}: {
	isConnectionParamsLoading?: boolean;
	connectionParams?: CloudintegrationtypesCredentialsDTO | null;
	isFormDisabled?: boolean;
}): JSX.Element | null {
	if (
		isConnectionParamsLoading ||
		(!!connectionParams?.ingestion_url &&
			!!connectionParams?.ingestion_key &&
			!!connectionParams?.observe_api_url &&
			!!connectionParams?.observe_api_key)
	) {
		return null;
	}

	return (
		<Form.Item name="connectionParams">
			{!connectionParams?.ingestionUrl && (
				<Form.Item
					name="ingestionUrl"
					label="Ingestion URL"
					rules={[{ required: true, message: 'Please enter ingestion URL' }]}
				>
					<Input placeholder="Enter ingestion URL" disabled={isFormDisabled} />
				</Form.Item>
			)}
			{!connectionParams?.ingestionKey && (
				<Form.Item
					name="ingestionKey"
					label="Ingestion Key"
					rules={[{ required: true, message: 'Please enter ingestion key' }]}
				>
					<Input placeholder="Enter ingestion key" disabled={isFormDisabled} />
				</Form.Item>
			)}
			{!connectionParams?.observe_api_url && (
				<Form.Item
					name="observe_api_url"
					label="Hanzo API URL"
					rules={[{ required: true, message: 'Please enter Hanzo API URL' }]}
				>
					<Input placeholder="Enter Hanzo API URL" disabled={isFormDisabled} />
				</Form.Item>
			)}
			{!connectionParams?.observe_api_key && (
				<Form.Item
					name="observe_api_key"
					label="Hanzo API KEY"
					rules={[{ required: true, message: 'Please enter Hanzo API Key' }]}
				>
					<Input placeholder="Enter Hanzo API Key" disabled={isFormDisabled} />
				</Form.Item>
			)}
		</Form.Item>
	);
}

RenderConnectionFields.defaultProps = {
	connectionParams: null,
	isFormDisabled: false,
	isConnectionParamsLoading: false,
};

export default RenderConnectionFields;
