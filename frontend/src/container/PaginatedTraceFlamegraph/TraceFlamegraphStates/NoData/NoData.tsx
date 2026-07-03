import { Typography } from 'components/ui/typography';

interface INoDataProps {
	id: string;
}

function NoData(props: INoDataProps): JSX.Element {
	const { id } = props;
	return <Typography.Text>No Trace found with the id: {id} </Typography.Text>;
}

export default NoData;
