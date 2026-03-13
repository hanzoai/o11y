import { useEffect, useState } from 'react';
import { useMutation, useQuery } from 'react-query';
import { toast } from '@hanzo/ui';
import type { NotificationInstance } from 'antd/es/notification/interface';
import logEvent from 'api/common/logEvent';
import { RenderErrorResponseDTO } from 'api/generated/services/o11y.schemas';
import { usePutProfile } from 'api/generated/services/zeus';
import listOrgPreferences from 'api/v1/org/preferences/list';
import updateOrgPreferenceAPI from 'api/v1/org/preferences/name/update';
import { AxiosError } from 'axios';
import { SOMETHING_WENT_WRONG } from 'constants/api';
import { FeatureKeys } from 'constants/features';
import { ORG_PREFERENCES } from 'constants/orgPreferences';
import ROUTES from 'constants/routes';
import { InviteTeamMembersProps } from 'container/OrganizationSettings/utils';
import { useNotifications } from 'hooks/useNotifications';
import history from 'lib/history';
import { useAppContext } from 'providers/App/App';

import {
	AboutHanzoQuestions,
	O11yDetails,
} from './AboutO11yQuestions/AboutO11yQuestions';
import InviteTeamMembers from './InviteTeamMembers/InviteTeamMembers';
import OptimiseO11yNeeds, {
	OptimiseO11yDetails,
} from './OptimiseO11yNeeds/OptimiseO11yNeeds';
import OrgQuestions, { OrgDetails } from './OrgQuestions/OrgQuestions';

import './OnboardingQuestionaire.styles.scss';

export const showErrorNotification = (
	notifications: NotificationInstance,
	err: Error,
): void => {
	notifications.error({
		message: err.message || SOMETHING_WENT_WRONG,
	});
};

const INITIAL_ORG_DETAILS: OrgDetails = {
	usesObservability: true,
	observabilityTool: '',
	otherTool: '',
	usesOtel: null,
	migrationTimeline: null,
};

const INITIAL_HANZO_DETAILS: O11yDetails = {
	interestInO11y: [],
	otherInterestInO11y: '',
	discoverO11y: '',
};

const INITIAL_OPTIMISE_HANZO_DETAILS: OptimiseO11yDetails = {
	logsPerDay: 0,
	hostsPerDay: 0,
	services: 0,
};

const NEXT_BUTTON_EVENT_NAME = 'Org Onboarding: Next Button Clicked';
const ONBOARDING_COMPLETE_EVENT_NAME = 'Org Onboarding: Complete';

function OnboardingQuestionaire(): JSX.Element {
	const { notifications } = useNotifications();
	const { org, updateOrgPreferences, featureFlags } = useAppContext();
	const isOnboardingV3Enabled = featureFlags?.find(
		(flag) => flag.name === FeatureKeys.ONBOARDING_V3,
	)?.active;
	const [currentStep, setCurrentStep] = useState<number>(1);
	const [orgDetails, setOrgDetails] = useState<OrgDetails>(INITIAL_ORG_DETAILS);
	const [o11yDetails, setO11yDetails] = useState<O11yDetails>(
		INITIAL_HANZO_DETAILS,
	);

	const [
		optimiseO11yDetails,
		setOptimiseO11yDetails,
	] = useState<OptimiseO11yDetails>(INITIAL_OPTIMISE_HANZO_DETAILS);
	const [teamMembers, setTeamMembers] = useState<
		InviteTeamMembersProps[] | null
	>(null);

	const [
		updatingOrgOnboardingStatus,
		setUpdatingOrgOnboardingStatus,
	] = useState<boolean>(false);

	useEffect(() => {
		logEvent('Org Onboarding: Started', {
			org_id: org?.[0]?.id,
		});
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const { refetch: refetchOrgPreferences } = useQuery({
		queryFn: () => listOrgPreferences(),
		queryKey: ['getOrgPreferences'],
		enabled: false,
		refetchOnWindowFocus: false,
		onSuccess: (response) => {
			if (response.data) {
				updateOrgPreferences(response.data);
			}

			setUpdatingOrgOnboardingStatus(false);

			logEvent('Org Onboarding: Redirecting to Get Started', {});

			if (isOnboardingV3Enabled) {
				history.push(ROUTES.GET_STARTED_WITH_CLOUD);
			} else {
				history.push(ROUTES.GET_STARTED);
			}
		},
		onError: () => {
			setUpdatingOrgOnboardingStatus(false);
		},
	});

	const isNextDisabled =
		optimiseO11yDetails.logsPerDay === 0 &&
		optimiseO11yDetails.hostsPerDay === 0 &&
		optimiseO11yDetails.services === 0;

	const { mutate: updateProfile, isLoading: isUpdatingProfile } = usePutProfile<
		AxiosError<RenderErrorResponseDTO>
	>();

	const { mutate: updateOrgPreference } = useMutation(updateOrgPreferenceAPI, {
		onSuccess: () => {
			refetchOrgPreferences();
		},
		onError: (error) => {
			showErrorNotification(notifications, error as AxiosError);

			setUpdatingOrgOnboardingStatus(false);
		},
	});

	const handleUpdateProfile = (): void => {
		logEvent(NEXT_BUTTON_EVENT_NAME, {
			currentPageID: 3,
			nextPageID: 4,
		});

		updateProfile(
			{
				data: {
					uses_otel: orgDetails?.usesOtel as boolean,
					has_existing_observability_tool: orgDetails?.usesObservability as boolean,
					existing_observability_tool:
						orgDetails?.observabilityTool === 'Others'
							? (orgDetails?.otherTool as string)
							: (orgDetails?.observabilityTool as string),
					where_did_you_discover_o11y: o11yDetails?.discoverO11y as string,
					timeline_for_migrating_to_o11y: orgDetails?.migrationTimeline as string,
					reasons_for_interest_in_o11y: o11yDetails?.interestInO11y?.includes(
						'Others',
					)
						? ([
								...(o11yDetails?.interestInO11y?.filter(
									(item) => item !== 'Others',
								) || []),
								o11yDetails?.otherInterestInO11y,
						  ] as string[])
						: (o11yDetails?.interestInO11y as string[]),
					logs_scale_per_day_in_gb: optimiseO11yDetails?.logsPerDay as number,
					number_of_hosts: optimiseO11yDetails?.hostsPerDay as number,
					number_of_services: optimiseO11yDetails?.services as number,
				},
			},
			{
				onSuccess: () => {
					setCurrentStep(4);
				},
				onError: (error: any) => {
					toast.error(error?.message || SOMETHING_WENT_WRONG);

					// Allow user to proceed even if API fails
					setCurrentStep(4);
				},
			},
		);
	};

	const handleOnboardingComplete = (): void => {
		logEvent(ONBOARDING_COMPLETE_EVENT_NAME, {
			currentPageID: 4,
		});

		setUpdatingOrgOnboardingStatus(true);
		updateOrgPreference({
			name: ORG_PREFERENCES.ORG_ONBOARDING,
			value: true,
		});
	};

	return (
		<div className="onboarding-questionaire-container">
			<div className="onboarding-questionaire-content">
				{currentStep === 1 && (
					<OrgQuestions
						orgDetails={{
							...orgDetails,
							usesOtel: orgDetails.usesOtel ?? null,
						}}
						onNext={(orgDetails: OrgDetails): void => {
							logEvent(NEXT_BUTTON_EVENT_NAME, {
								currentPageID: 1,
								nextPageID: 2,
							});

							setOrgDetails(orgDetails);
							setCurrentStep(2);
						}}
					/>
				)}

				{currentStep === 2 && (
					<AboutHanzoQuestions
						o11yDetails={o11yDetails}
						setO11yDetails={setO11yDetails}
						onNext={(): void => {
							logEvent(NEXT_BUTTON_EVENT_NAME, {
								currentPageID: 2,
								nextPageID: 3,
							});
							setCurrentStep(3);
						}}
					/>
				)}

				{currentStep === 3 && (
					<OptimiseO11yNeeds
						isNextDisabled={isNextDisabled}
						isUpdatingProfile={isUpdatingProfile}
						optimiseO11yDetails={optimiseO11yDetails}
						setOptimiseO11yDetails={setOptimiseO11yDetails}
						onNext={handleUpdateProfile}
						onWillDoLater={handleUpdateProfile}
					/>
				)}

				{currentStep === 4 && (
					<InviteTeamMembers
						isLoading={updatingOrgOnboardingStatus}
						teamMembers={teamMembers}
						setTeamMembers={setTeamMembers}
						onNext={handleOnboardingComplete}
					/>
				)}
			</div>
		</div>
	);
}

export default OnboardingQuestionaire;
