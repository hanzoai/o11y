import { useEffect } from 'react';
import { Button, Col, Modal, Row, Skeleton, Space, Typography } from 'antd';
import ROUTES from 'constants/routes';
import history from 'lib/history';
import { useAppContext } from 'providers/App/App';
import { LicensePlatform, LicenseState } from 'types/api/licensesV3/getActive';

import './WorkspaceAccessRestricted.styles.scss';

function WorkspaceAccessRestricted(): JSX.Element {
	const { activeLicense, isFetchingActiveLicense } = useAppContext();

	useEffect(() => {
		if (!isFetchingActiveLicense) {
			const isTerminated = activeLicense?.state === LicenseState.TERMINATED;
			const isExpired = activeLicense?.state === LicenseState.EXPIRED;
			const isCancelled = activeLicense?.state === LicenseState.CANCELLED;

			const isWorkspaceAccessRestricted = isTerminated || isExpired || isCancelled;

			if (
				!isWorkspaceAccessRestricted ||
				activeLicense.platform === LicensePlatform.SELF_HOSTED
			) {
				history.push(ROUTES.HOME);
			}
		}
	}, [isFetchingActiveLicense, activeLicense]);

	return (
		<div>
			<Modal
				rootClassName="workspace-access-restricted__modal"
				title={
					<div className="workspace-access-restricted__modal__header">
						<span className="workspace-access-restricted__modal__title">
							Your workspace access is restricted
						</span>
					</div>
				}
				open
				closable={false}
				footer={null}
				width="65%"
			>
				<div className="workspace-access-restricted__container">
					{isFetchingActiveLicense || !activeLicense ? (
						<Skeleton />
					) : (
						<>
							<Row justify="center" align="middle">
								<Col>
									<Space direction="vertical" align="center">
										<Typography.Title
											level={4}
											className="workspace-access-restricted__details"
										>
											{activeLicense.state === LicenseState.TERMINATED && (
												<>
													Your HanzoO11y license is terminated, please contact support at{' '}
													<a href="mailto:cloud-support@o11y.hanzo.ai">
														cloud-support@o11y.hanzo.ai
													</a>{' '}
													for a new deployment
												</>
											)}
											{activeLicense.state === LicenseState.EXPIRED && (
												<>
													Your HanzoO11y license is expired, please contact support at{' '}
													<a href="mailto:cloud-support@o11y.hanzo.ai">
														cloud-support@o11y.hanzo.ai
													</a>{' '}
													for renewal to avoid termination of license as per our{' '}
													<a
														href="https://o11y.hanzo.ai/terms-of-service"
														target="_blank"
														rel="noopener noreferrer"
													>
														terms of service
													</a>
													.
												</>
											)}
											{activeLicense.state === LicenseState.CANCELLED && (
												<>
													Your HanzoO11y license is cancelled, please contact support at{' '}
													<a href="mailto:cloud-support@o11y.hanzo.ai">
														cloud-support@o11y.hanzo.ai
													</a>{' '}
													for reactivation to avoid termination of license as per our{' '}
													<a
														href="https://o11y.hanzo.ai/terms-of-service"
														target="_blank"
														rel="noopener noreferrer"
													>
														terms of service
													</a>
													.
												</>
											)}
										</Typography.Title>

										<Button
											type="default"
											shape="round"
											size="middle"
											href="mailto:cloud-support@o11y.hanzo.ai"
											role="button"
										>
											Contact Us
										</Button>
									</Space>
								</Col>
							</Row>
							<div className="workspace-access-restricted__creative">
								<img
									src="/Images/feature-graphic-correlation.svg"
									alt="correlation-graphic"
								/>
							</div>
						</>
					)}
				</div>
			</Modal>
		</div>
	);
}

export default WorkspaceAccessRestricted;
