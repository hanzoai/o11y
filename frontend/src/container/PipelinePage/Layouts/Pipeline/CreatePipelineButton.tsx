import { useMemo, type JSX } from 'react';
import { useTranslation } from 'react-i18next';
import logEvent from 'api/common/logEvent';
import TextToolTip from 'components/TextToolTip';
import { ActionMode, ActionType, Pipeline } from 'types/api/pipeline/def';

import { ButtonContainer, CustomButton } from '../../styles';
import { checkDataLength } from '../utils';
import { PencilLine, Plus } from 'components/ui/icons';
import { Flex } from 'antd';

function CreatePipelineButton({
	setActionType,
	isActionMode,
	setActionMode,
	pipelineData,
}: CreatePipelineButtonProps): JSX.Element {
	const { t } = useTranslation(['pipeline']);

	const isAddNewPipelineVisible = useMemo(
		() => checkDataLength(pipelineData?.pipelines),
		[pipelineData?.pipelines],
	);
	const isDisabled = isActionMode === ActionMode.Editing;

	const onEnterEditMode = (): void => {
		setActionMode(ActionMode.Editing);

		logEvent('Logs: Pipelines: Entered Edit Mode', {
			source: 'observe-ui',
		});
	};
	const onAddNewPipeline = (): void => {
		setActionMode(ActionMode.Editing);
		setActionType(ActionType.AddPipeline);

		logEvent('Logs: Pipelines: Clicked Add New Pipeline', {
			source: 'observe-ui',
		});
	};

	return (
		<ButtonContainer>
			<TextToolTip
				text={t('learn_more')}
				url="https://o11y.hanzo.ai/docs/logs-pipelines/introduction/?utm_source=product&utm_medium=pipelines-tab"
			/>
			{isAddNewPipelineVisible && (
				<CustomButton onClick={onEnterEditMode} disabled={isDisabled}>
					<Flex align="center" gap={4}>
						<PencilLine size={16} />
						{t('enter_edit_mode')}
					</Flex>
				</CustomButton>
			)}
			{!isAddNewPipelineVisible && (
				<CustomButton icon={<Plus />} onClick={onAddNewPipeline} type="primary">
					{t('new_pipeline')}
				</CustomButton>
			)}
		</ButtonContainer>
	);
}

interface CreatePipelineButtonProps {
	setActionType: (actionType: string) => void;
	isActionMode: string;
	setActionMode: (actionMode: string) => void;
	pipelineData: Pipeline;
}

export default CreatePipelineButton;
