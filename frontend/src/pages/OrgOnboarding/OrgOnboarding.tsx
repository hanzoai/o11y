import AuthPageContainer from 'components/AuthPageContainer';
import OnboardingQuestionaire from 'container/OnboardingQuestionaire';

import type { JSX } from 'react';

function OrgOnboarding(): JSX.Element {
	return (
		<AuthPageContainer isOnboarding>
			<OnboardingQuestionaire />
		</AuthPageContainer>
	);
}

export default OrgOnboarding;
