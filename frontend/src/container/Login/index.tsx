import { useEffect, useState } from 'react';
import { useQuery } from 'react-query';
import { Typography } from 'antd';
import getVersion from 'api/v1/version/get';
import get from 'api/v2/sessions/context/get';
import afterLogin from 'AppRoutes/utils';
import AuthError from 'components/AuthError/AuthError';
import BrandMark from 'components/BrandMark';
import ROUTES from 'constants/routes';
import useUrlQuery from 'hooks/useUrlQuery';
import history from 'lib/history';
import { useTenant } from 'providers/Tenant';
import APIError from 'types/api/error';

import './Login.styles.scss';

function parseErrors(errors: string): { message: string }[] {
	try {
		return JSON.parse(errors).map((e: { message: string }) => ({
			message: e.message,
		}));
	} catch {
		return [];
	}
}

function Login(): JSX.Element {
	const tenant = useTenant();
	const urlQueryParams = useUrlQuery();

	// Tokens returned from IAM OIDC callback
	const accessToken = urlQueryParams.get('accessToken') || '';
	const refreshToken = urlQueryParams.get('refreshToken') || '';

	// OIDC callback error handling
	const callbackAuthError = urlQueryParams.get('callbackauthnerr') || '';
	const callbackAuthErrorCode = urlQueryParams.get('code') || '';
	const callbackAuthErrorMessage = urlQueryParams.get('message') || '';
	const callbackAuthErrorURL = urlQueryParams.get('url') || '';
	const callbackAuthErrorAdditional = urlQueryParams.get('errors') || '';

	const [errorMessage, setErrorMessage] = useState<APIError>();
	const [redirecting, setRedirecting] = useState(false);

	// Check setup status
	const {
		data: versionData,
		isLoading: versionLoading,
		error: versionError,
	} = useQuery({
		queryFn: getVersion,
		queryKey: ['api/v1/version/get'],
		enabled: true,
	});

	// Route to signup if setup incomplete
	useEffect(() => {
		if (
			versionData &&
			!versionLoading &&
			!versionError &&
			!versionData.data.setupCompleted
		) {
			history.push(ROUTES.SIGN_UP);
		}
	}, [versionData, versionLoading, versionError]);

	// Handle tokens returned from OIDC callback
	useEffect(() => {
		if (accessToken && refreshToken) {
			afterLogin(accessToken, refreshToken);
		}
	}, [accessToken, refreshToken]);

	// Handle OIDC callback errors
	useEffect(() => {
		if (callbackAuthError) {
			setErrorMessage(
				new APIError({
					httpStatusCode: 500,
					error: {
						code: callbackAuthErrorCode,
						message: callbackAuthErrorMessage,
						url: callbackAuthErrorURL,
						errors: parseErrors(callbackAuthErrorAdditional),
					},
				}),
			);
		}
	}, [
		callbackAuthError,
		callbackAuthErrorAdditional,
		callbackAuthErrorCode,
		callbackAuthErrorMessage,
		callbackAuthErrorURL,
	]);

	// Auto-redirect to IAM OIDC — fetch the correct authorize URL from backend
	useEffect(() => {
		if (accessToken || refreshToken || callbackAuthError || redirecting) {
			return;
		}
		if (versionLoading || !versionData) {
			return;
		}
		if (!versionData.data.setupCompleted) {
			return;
		}

		setRedirecting(true);

		// Use the tenant's org slug domain for OIDC probe
		const probeDomain = tenant.orgSlug ? `login@${tenant.orgSlug}.ai` : 'login@o11y.local';
		get({ email: probeDomain, ref: window.location.origin })
			.then((response) => {
				const orgs = response.data.orgs;
				if (orgs.length > 0) {
					const org = orgs[0];
					const callbacks = org.authNSupport?.callback || [];
					if (callbacks.length > 0) {
						// Redirect to IAM
						window.location.href = callbacks[0].url;
						return;
					}
				}
				// Fallback: no OIDC configured, show error
				setErrorMessage(
					new APIError({
						httpStatusCode: 500,
						error: {
							code: 'no_sso',
							message:
								'No SSO provider configured. Contact your administrator.',
							url: '',
							errors: [],
						},
					}),
				);
				setRedirecting(false);
			})
			.catch(() => {
				setErrorMessage(
					new APIError({
						httpStatusCode: 500,
						error: {
							code: 'login_error',
							message: 'Unable to initiate SSO login. Please try again.',
							url: '',
							errors: [],
						},
					}),
				);
				setRedirecting(false);
			});
	}, [
		accessToken,
		refreshToken,
		callbackAuthError,
		versionLoading,
		versionData,
		redirecting,
		tenant.orgSlug,
	]);

	// Error state — show error with retry link
	if (errorMessage) {
		return (
			<div className="login-form-container">
				<div className="login-form-header">
					<div className="login-form-emoji">
						<BrandMark size={32} showProduct />
					</div>
				</div>
				<AuthError error={errorMessage} />
				<div className="login-form-actions" style={{ marginTop: 16 }}>
					<a
						href="/login"
						style={{
							color: '#fff',
							textDecoration: 'underline',
							fontSize: 14,
						}}
					>
						Try again
					</a>
				</div>
			</div>
		);
	}

	// Loading / redirecting state
	return (
		<div className="login-form-container">
			<div className="login-form-header">
				<div className="login-form-emoji">
					<BrandMark size={32} showProduct />
				</div>
				<Typography.Paragraph className="login-form-description">
					Redirecting to login...
				</Typography.Paragraph>
			</div>
		</div>
	);
}

export default Login;
