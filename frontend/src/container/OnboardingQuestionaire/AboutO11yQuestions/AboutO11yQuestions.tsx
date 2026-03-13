import { useEffect, useState } from 'react';
import { Button } from '@o11yhq/button';
import { Checkbox } from '@o11yhq/checkbox';
import { Input } from '@o11yhq/input';
import { Input as AntdInput } from 'antd';
import logEvent from 'api/common/logEvent';
import { ArrowRight } from 'lucide-react';

import { OnboardingQuestionHeader } from '../OnboardingQuestionHeader';

import '../OnboardingQuestionaire.styles.scss';

export interface O11yDetails {
	interestInO11y: string[] | null;
	otherInterestInO11y: string | null;
	discoverO11y: string | null;
}

interface AboutHanzoQuestionsProps {
	o11yDetails: O11yDetails;
	setO11yDetails: (details: O11yDetails) => void;
	onNext: () => void;
}

const interestedInOptions: Record<string, string> = {
	loweringCosts: 'Lowering observability costs',
	otelNativeStack: 'Interested in OTel-native stack',
	deploymentFlexibility: 'Deployment flexibility (Cloud/Self-Host) in future',
	singleTool:
		'Single Tool (logs, metrics & traces) to reduce operational overhead',
	correlateSignals: 'Correlate signals for faster troubleshooting',
};

export function AboutHanzoQuestions({
	o11yDetails,
	setO11yDetails,
	onNext,
}: AboutHanzoQuestionsProps): JSX.Element {
	const [interestInO11y, setInterestInO11y] = useState<string[]>(
		o11yDetails?.interestInO11y || [],
	);
	const [otherInterestInO11y, setOtherInterestInO11y] = useState<string>(
		o11yDetails?.otherInterestInO11y || '',
	);
	const [discoverO11y, setDiscoverO11y] = useState<string>(
		o11yDetails?.discoverO11y || '',
	);
	const [isNextDisabled, setIsNextDisabled] = useState<boolean>(true);

	useEffect((): void => {
		if (
			discoverO11y !== '' &&
			interestInO11y.length > 0 &&
			(!interestInO11y.includes('Others') || otherInterestInO11y !== '')
		) {
			setIsNextDisabled(false);
		} else {
			setIsNextDisabled(true);
		}
	}, [interestInO11y, otherInterestInO11y, discoverO11y]);

	const handleInterestChange = (option: string, checked: boolean): void => {
		if (checked) {
			setInterestInO11y((prev) => [...prev, option]);
		} else {
			setInterestInO11y((prev) => prev.filter((item) => item !== option));
		}
	};

	const createInterestChangeHandler = (option: string) => (
		checked: boolean,
	): void => {
		handleInterestChange(option, Boolean(checked));
	};

	const handleOnNext = (): void => {
		setO11yDetails({
			discoverO11y,
			interestInO11y,
			otherInterestInO11y,
		});

		logEvent('Org Onboarding: Answered', {
			discoverO11y,
			interestInO11y,
			otherInterestInO11y,
		});

		onNext();
	};

	return (
		<div className="questions-container">
			<OnboardingQuestionHeader
				title="Set up your workspace"
				subtitle="Tailor Hanzo to suit your observability needs."
			/>

			<div className="questions-form-container">
				<div className="questions-form">
					<div className="form-group">
						<div className="question">How did you first come across Hanzo?</div>

						<AntdInput.TextArea
							className="discover-o11y-input"
							placeholder={`e.g., googling "datadog alternative", a post on r/devops, from a friend/colleague, a LinkedIn post, ChatGPT, etc.`}
							value={discoverO11y}
							autoFocus
							rows={4}
							onChange={(e): void => setDiscoverO11y(e.target.value)}
						/>
					</div>

					<div className="form-group">
						<div className="question">What got you interested in Hanzo?</div>
						<div className="checkbox-grid">
							{Object.keys(interestedInOptions).map((option: string) => (
								<div key={option} className="checkbox-item">
									<Checkbox
										id={`checkbox-${option}`}
										checked={interestInO11y.includes(option)}
										onCheckedChange={createInterestChangeHandler(option)}
										labelName={interestedInOptions[option]}
									/>
								</div>
							))}

							<div className="checkbox-item checkbox-item-others">
								<Checkbox
									id="others-checkbox"
									checked={interestInO11y.includes('Others')}
									onCheckedChange={createInterestChangeHandler('Others')}
									labelName={interestInO11y.includes('Others') ? '' : 'Others'}
								/>
								{interestInO11y.includes('Others') && (
									<Input
										type="text"
										className="onboarding-questionaire-other-input"
										placeholder="What got you interested in Hanzo?"
										value={otherInterestInO11y}
										autoFocus
										onChange={(e): void => setOtherInterestInO11y(e.target.value)}
									/>
								)}
							</div>
						</div>
					</div>
				</div>

				<div className="onboarding-buttons-container">
					<Button
						variant="solid"
						color="primary"
						className={`onboarding-next-button ${isNextDisabled ? 'disabled' : ''}`}
						onClick={handleOnNext}
						disabled={isNextDisabled}
						suffixIcon={<ArrowRight size={12} />}
					>
						Next
					</Button>
				</div>
			</div>
		</div>
	);
}
